package fetch

import (
	"context"
	"fmt"
	"time"

	"github.com/dnldd/entry/market"
	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron/v2"
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
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager represents the market query manager.
type Manager struct {
	cfg              *ManagerConfig
	lastUpdatedTimes map[string]time.Time
	jobScheduler     *gocron.Scheduler
	catchUpSignals   chan market.CatchUpSignal
	subscribers      []*chan shared.Candlestick
	workers          chan struct{}
}

// NewManager initializes the query manager.
func NewQueryManager(cfg *ManagerConfig) (*Manager, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("creating job scheduler: %w", err)
	}

	mgr := &Manager{
		cfg:              cfg,
		lastUpdatedTimes: make(map[string]time.Time),
		jobScheduler:     &scheduler,
		catchUpSignals:   make(chan market.CatchUpSignal, bufferSize),
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
func (m *Manager) SendCatchUpSignal(catchUp market.CatchUpSignal) {
	select {
	case m.catchUpSignals <- catchUp:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("catchup signal channel at capacity: %d/%d",
			len(m.catchUpSignals), bufferSize)
	}
}

// handleCatchUpSignal processes the provided catch up signal.
func (m *Manager) handleCatchUpSignal(signal market.CatchUpSignal) {
	data, err := m.cfg.ExchangeClient.FetchIndexIntradayHistorical(context.Background(), signal.Market,
		signal.Timeframe, signal.Start, time.Time{})
	if err != nil {
		m.cfg.Logger.Error().Msgf("catching up on %s: %v", signal.Market, err)
		return
	}

	candles, err := m.cfg.ExchangeClient.ParseCandlesticks(data, signal.Market, signal.Timeframe)
	if err != nil {
		m.cfg.Logger.Error().Msgf("parsing candlesticks for %s: %v", signal.Market, err)
		return
	}

	for idx := range candles {
		m.notifySubscribers(&candles[idx])
	}

	m.lastUpdatedTimes[signal.Market] = candles[len(candles)-1].Date

	// todo: set up periodic market data fetches on regular intervals, only proceeding when
	// the market is open.
}

// Run  manages the lifecycle processes of the query manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case signal := <-m.catchUpSignals:
			m.workers <- struct{}{}
			go func(signal *market.CatchUpSignal) {
				m.handleCatchUpSignal(*signal)
				<-m.workers
			}(&signal)
		default:
			// fallthrough
		}
	}
}
