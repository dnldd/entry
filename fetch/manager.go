package fetch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
	// minSubscriberBuffer is the minimum buffer size for subscribers.
	minSubscriberBuffer = 24
	// notifyTimeout is the maximum time to wait before timing out for a market update notification.
	notifyTimeout = time.Second * 3
)

// ManagerConfig represents the configuration for the query manager.
type ManagerConfig struct {
	// Markets represents the tracked markets.
	Markets []string
	// ExchangeClient represents the market exchange client.
	ExchangeClient shared.MarketFetcher
	// SignalCaughtUp signals a market is caught up on market data.
	SignalCaughtUp func(signal shared.CaughtUpSignal)
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager represents the market query manager.
type Manager struct {
	cfg                 *ManagerConfig
	lastUpdatedTimes    map[string]time.Time
	lastUpdatedTimesMtx sync.RWMutex
	catchUpSignals      chan shared.CatchUpSignal
	subscribers         map[string]chan shared.Candlestick
	subscribersMtx      sync.RWMutex
	location            *time.Location
	workers             chan struct{}
	timer               *time.Timer
}

// NewManager initializes the fetch manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	// Track the last updated times for all expected markets.
	lastUpdatedTimes := make(map[string]time.Time)
	for idx := range cfg.Markets {
		lastUpdatedTimes[cfg.Markets[idx]] = time.Time{}
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	if err != nil {
		return nil, fmt.Errorf("loading new york location: %v", err)
	}

	timer := time.NewTimer(notifyTimeout)
	timer.Stop()

	mgr := &Manager{
		cfg:              cfg,
		lastUpdatedTimes: lastUpdatedTimes,
		catchUpSignals:   make(chan shared.CatchUpSignal, bufferSize),
		subscribers:      make(map[string]chan shared.Candlestick),
		workers:          make(chan struct{}, maxWorkers),
		location:         loc,
		timer:            timer,
	}

	return mgr, nil
}

// Subscriber registers the provided subscriber for market updates.
func (m *Manager) Subscribe(name string, sub chan shared.Candlestick) {
	m.subscribersMtx.Lock()
	m.subscribers[name] = sub
	m.subscribersMtx.Unlock()
}

// notifySubscribers notifies subscribers of the new market update.
func (m *Manager) NotifySubscribers(candle shared.Candlestick) error {
	m.subscribersMtx.RLock()
	defer m.subscribersMtx.RUnlock()
	subs := len(m.subscribers)

	// Notify subscribers.
	for k := range m.subscribers {
		m.subscribers[k] <- candle
	}

	// Wait for subscribers to process the candle.
	m.timer.Reset(notifyTimeout)
	for range subs {
		select {
		case <-candle.Status:
			m.timer.Stop()
		case <-m.timer.C:
			m.timer.Stop()
			return fmt.Errorf("timed out waiting for market update processing")
		}
	}

	m.cfg.Logger.Info().Msgf("processed candle update – %v", candle.Date)

	return nil
}

// SendCatchUpSignal relays the provided market catch up signal for processing.
func (m *Manager) SendCatchUpSignal(catchUp shared.CatchUpSignal) {
	select {
	case m.catchUpSignals <- catchUp:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("catchup signal channel at capacity: %d/%d",
			len(m.catchUpSignals), bufferSize)
	}
}

// fetchMarketData fetches market data using the provided parameters.
func (m *Manager) fetchMarketData(market string, timeframe shared.Timeframe, start time.Time) error {
	data, err := m.cfg.ExchangeClient.FetchIndexIntradayHistorical(context.Background(), market,
		timeframe, start, time.Time{})
	if err != nil {
		return fmt.Errorf("fetching market data %s: %v", market, err)
	}

	candles, err := shared.ParseCandlesticks(data, market, timeframe, m.location)
	if err != nil {
		return fmt.Errorf("parsing candlesticks for %s: %v", market, err)
	}

	for idx := range candles {
		m.NotifySubscribers(candles[idx])
	}

	key := shared.MarketDataKey(market, timeframe.String())
	m.lastUpdatedTimesMtx.Lock()
	m.lastUpdatedTimes[key] = candles[len(candles)-1].Date
	m.lastUpdatedTimesMtx.Unlock()

	return nil
}

// fetchMatketDataJob is a job used to fetch market data using the provided parameters.
//
// This job should be scheduled for periodic execution.
func (m *Manager) fetchMarketDataJob(marketName string, timeframe shared.Timeframe) error {
	key := shared.MarketDataKey(marketName, timeframe.String())

	m.lastUpdatedTimesMtx.Lock()
	lastUpdatedTime, ok := m.lastUpdatedTimes[key]
	m.lastUpdatedTimesMtx.Unlock()

	// A market is required to be caught up and have a last updated time in order to receive
	// periodic market updates.
	if !ok {
		return fmt.Errorf("no last updated time found for %s, skipping market %s update",
			marketName, timeframe.String())
	}

	// Avoid fetching periodic market data if the market is not open.
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return fmt.Errorf("creating new york time: %v", err)
	}

	open, _, err := shared.IsMarketOpen(now)
	if err != nil {
		return fmt.Errorf("checking market open status: %v", err)
	}

	if !open {
		// do nothing.
		return fmt.Errorf("%s not open, skipping periodic update", marketName)
	}

	m.fetchMarketData(marketName, timeframe, lastUpdatedTime)

	return nil
}

// handleCatchUpSignal processes the provided catch up signal.
func (m *Manager) handleCatchUpSignal(signal shared.CatchUpSignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	key := shared.MarketDataKey(signal.Market, signal.Timeframe.String())
	m.lastUpdatedTimesMtx.RLock()
	_, ok := m.lastUpdatedTimes[key]
	m.lastUpdatedTimesMtx.RUnlock()

	if !ok {
		return fmt.Errorf("unexpected catch up signal for market %s with timeframe %s",
			signal.Market, signal.Timeframe.String())
	}

	m.fetchMarketData(signal.Market, signal.Timeframe, signal.Start)

	sig := shared.NewCaughtUpSignal(signal.Market)
	m.cfg.SignalCaughtUp(sig)

	// Periodically fetch market updates once caught up.
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return fmt.Errorf("fetching new york time: %v", err)
	}

	startTime, err := shared.NextInterval(signal.Timeframe, now)
	if err != nil {
		return fmt.Errorf("fetching next %s interval time: %v", signal.Timeframe.String(), err)
	}

	// Add a few seconds to ensure a market update occurs before the job runs.
	startTime = startTime.Add(time.Second * 5)
	var jobIntervalSeconds uint32
	switch signal.Timeframe {
	case shared.OneMinute:
		jobIntervalSeconds = 65
	case shared.FiveMinute:
		jobIntervalSeconds = 305
	}

	_, err = m.cfg.JobScheduler.Every(jobIntervalSeconds).Seconds().StartAt(startTime).
		Do(func() {
			err := m.fetchMarketDataJob(signal.Market, signal.Timeframe)
			if err != nil {
				m.cfg.Logger.Error().Err(err).Send()
			}
		})
	if err != nil {
		return fmt.Errorf("scheduling %s market update job for %s: %v", signal.Market,
			signal.Timeframe.String(), err)
	}

	return nil
}

// Run manages the lifecycle processes of the query manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-m.catchUpSignals:
			m.workers <- struct{}{}
			go func(signal shared.CatchUpSignal) {
				err := m.handleCatchUpSignal(signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(signal)
		default:
			// fallthrough
		}
	}
}
