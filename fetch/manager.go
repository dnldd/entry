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
)

// ManagerConfig represents the configuration for the query manager.
type ManagerConfig struct {
	// ExchangeClient represents the market exchange client.
	ExchangeClient *FMPClient
	// SignalCaughtUp signals a market is caught up on market data.
	SignalCaughtUp func(signal shared.CaughtUpSignal)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager represents the market query manager.
type Manager struct {
	cfg                 *ManagerConfig
	lastUpdatedTimes    map[string]time.Time
	lastUpdatedTimesMtx sync.RWMutex
	jobScheduler        *gocron.Scheduler
	catchUpSignals      chan shared.CatchUpSignal

	subscribers []*chan shared.Candlestick
	workers     chan struct{}
}

// NewManager initializes the query manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("loading new york timezone: %w", err)
	}

	scheduler := gocron.NewScheduler(loc)

	mgr := &Manager{
		cfg:              cfg,
		lastUpdatedTimes: make(map[string]time.Time),
		jobScheduler:     scheduler,
		catchUpSignals:   make(chan shared.CatchUpSignal, bufferSize),
		subscribers:      make([]*chan shared.Candlestick, 0, minSubscriberBuffer),
		workers:          make(chan struct{}),
	}

	return mgr, nil
}

// Subscriber registers the provided subscriber for market updates.
func (m *Manager) Subscribe(sub *chan shared.Candlestick) {
	m.subscribers = append(m.subscribers, sub)
}

// notifySubscribers notifies subscribers of the new market update.
func (m *Manager) notifySubscribers(candle *shared.Candlestick) {
	for k := range m.subscribers {
		*m.subscribers[k] <- *candle
	}
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
func (m *Manager) fetchMarketData(market string, timeframe shared.Timeframe, start time.Time) {
	data, err := m.cfg.ExchangeClient.FetchIndexIntradayHistorical(context.Background(), market,
		timeframe, start, time.Time{})
	if err != nil {
		m.cfg.Logger.Error().Msgf("fetching market data %s: %v", market, err)
		return
	}

	candles, err := m.cfg.ExchangeClient.ParseCandlesticks(data, market, timeframe)
	if err != nil {
		m.cfg.Logger.Error().Msgf("parsing candlesticks for %s: %v", market, err)
		return
	}

	for idx := range candles {
		m.notifySubscribers(&candles[idx])
	}

	m.lastUpdatedTimesMtx.Lock()
	m.lastUpdatedTimes[market] = candles[len(candles)-1].Date
	m.lastUpdatedTimesMtx.Unlock()
}

// fetchMatketDataJob is a job used to fetch market data using the provided parameters.
//
// This job should be scheduled for periodic execution.
func (m *Manager) fetchMarketDataJob(marketName string, timeframe shared.Timeframe) {
	m.lastUpdatedTimesMtx.Lock()
	lastUpdatedTime, ok := m.lastUpdatedTimes[marketName]
	m.lastUpdatedTimesMtx.Unlock()

	// A market is required to be caught up and have a last updated time in order to receive
	// periodic market updates.
	if !ok {
		m.cfg.Logger.Error().Msgf("no last updated time found for %s, skipping market %s update",
			marketName, timeframe.String())
		return
	}

	// Avoid fetching periodic market data if the market is not open.
	now, _, err := shared.NewYorkTime()
	if err != nil {
		m.cfg.Logger.Error().Msgf("creating new york time: %v", err)
	}

	open, _, err := shared.IsMarketOpen(now)
	if err != nil {
		m.cfg.Logger.Error().Msgf("checking market open status: %v", err)
	}

	if !open {
		// do nothing.
		m.cfg.Logger.Info().Msgf("%s not open, skipping periodic update", marketName)
		return
	}

	m.fetchMarketData(marketName, timeframe, lastUpdatedTime)
}

// handleCatchUpSignal processes the provided catch up signal.
func (m *Manager) handleCatchUpSignal(signal shared.CatchUpSignal) {
	defer func() {
		if signal.Done != nil {
			close(signal.Done)
		}
	}()

	m.fetchMarketData(signal.Market, signal.Timeframe, signal.Start)

	sig := shared.CaughtUpSignal{
		Market: signal.Market,
	}

	m.cfg.SignalCaughtUp(sig)

	_, err := m.jobScheduler.Every(5).Minutes().Do(m.fetchMarketDataJob, signal.Market, signal.Timeframe)
	if err != nil {
		m.cfg.Logger.Error().Msgf("scheduling %s market update job for %s: %v", signal.Market,
			signal.Timeframe.String(), err)
		return
	}
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
				m.handleCatchUpSignal(signal)
				<-m.workers
			}(signal)
		default:
			// fallthrough
		}
	}
}
