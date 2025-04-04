package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/rs/zerolog"
)

// QueryManagerConfig represents the configuration for the query manager.
type QueryManagerConfig struct {
	// ExchangeClient represents the market exchange client.
	ExchangeClient *FMPClient
	// SendMarketUpdate relays the provided candlestick for processing.
	SendMarketUpdate func(candle Candlestick)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// QueryManager represents the market query manager.
type QueryManager struct {
	cfg              *QueryManagerConfig
	lastUpdatedTimes map[string]time.Time
	jobScheduler     *gocron.Scheduler
	catchUpSignals   chan CatchUpSignal
	workers          chan struct{}
}

// NewQueryManager initializes the query manager.
func NewQueryManager(cfg *QueryManagerConfig) (*QueryManager, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("creating job scheduler: %w", err)
	}

	mgr := &QueryManager{
		cfg:              cfg,
		lastUpdatedTimes: make(map[string]time.Time),
		jobScheduler:     &scheduler,
		catchUpSignals:   make(chan CatchUpSignal, bufferSize),
		workers:          make(chan struct{}),
	}

	return mgr, nil
}

// SendCatchUpSignal relays the provided market catch up signal for processing.
func (m *QueryManager) SendCatchUpSignal(catchUp CatchUpSignal) {
	select {
	case m.catchUpSignals <- catchUp:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("catchup signal channel at capacity: %d/%d",
			len(m.catchUpSignals), bufferSize)
	}
}

// handleCatchUpSignal processes the provided catch up signal.
func (m *QueryManager) handleCatchUpSignal(signal CatchUpSignal) {
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
		m.cfg.SendMarketUpdate(candles[idx])
	}

	m.lastUpdatedTimes[signal.Market] = candles[len(candles)-1].Date

	// todo: set up periodic market data fetches on regular intervals, only proceeding when
	// the market is open.
}

// Run  manages the lifecycle processes of the query manager.
func (m *QueryManager) Run(ctx context.Context) {
	for {
		select {
		case signal := <-m.catchUpSignals:
			m.workers <- struct{}{}
			go func(signal *CatchUpSignal) {
				m.handleCatchUpSignal(*signal)
				<-m.workers
			}(&signal)
		default:
			// fallthrough
		}
	}
}
