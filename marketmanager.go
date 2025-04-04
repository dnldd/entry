package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe Timeframe
	Start     time.Time
}

// MarketManagerConfig represents the market manager configuration.
type MarketManagerConfig struct {
	// MarketIDs represetns the collection of ids of the markets to manage.
	MarketIDs []string
	// CatchUp signals a catchup process for a market.
	CatchUp func(signal CatchUpSignal)
	// TrackLevel signals the provided level to be tracked for price reactions.
	TrackLevel func(level Level)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// MarketManager manages the lifecycle processes of  all tracked markets.
type MarketManager struct {
	cfg           *MarketManagerConfig
	markets       map[string]*Market
	updateSignals chan Candlestick
	closeSignals  chan string
}

// NewMarketManager initializes a new market manager.
func NewMarketManager(cfg *MarketManagerConfig) (*MarketManager, error) {
	// initialize managed markets.
	markets := make(map[string]*Market, 0)
	for idx := range cfg.MarketIDs {
		market, err := NewMarket(cfg.MarketIDs[idx], cfg.TrackLevel)
		if err != nil {
			return nil, fmt.Errorf("creating market: %w", err)
		}
		markets[cfg.MarketIDs[idx]] = market
	}

	return &MarketManager{
		cfg:           cfg,
		markets:       markets,
		updateSignals: make(chan Candlestick, bufferSize),
		closeSignals:  make(chan string, bufferSize),
	}, nil
}

// SendMarketUpdate relays the provided candlestick for processing.
func (m *MarketManager) SendMarketUpdate(candle Candlestick) {
	select {
	case m.updateSignals <- candle:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market update channel at capacity: %d/%d",
			len(m.updateSignals), bufferSize)
	}
}

// SendMarketClose relays the provided closed market for processing.
func (m *MarketManager) SendMarketClose(market string) {
	select {
	case m.closeSignals <- market:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market close channel at capacity: %d/%d",
			len(m.closeSignals), bufferSize)
	}
}

// handleUpdateSignal processes the provided market update candle.
func (m *MarketManager) handleUpdateCandle(candle *Candlestick) {
	market, ok := m.markets[candle.Market]
	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s for update", candle.Market)
		return
	}

	err := market.Update(candle)
	if err != nil {
		m.cfg.Logger.Error().Msgf("updating %s market: %v", candle.Market, err)
		return
	}
}

// catchup signals a catch up for all tracked markets.
func (m *MarketManager) catchUp() {
	for _, v := range m.markets {
		market := *v

		signal := CatchUpSignal{
			Market:    market.Market,
			Timeframe: FiveMinute,
			Start:     market.FetchLastSessionOpen(),
		}

		m.cfg.CatchUp(signal)
	}
}

// Run manages the lifecycle processes of the position manager.
func (m *MarketManager) Run(ctx context.Context) {
	m.catchUp()

	for {
		select {
		// todo: add update signal workers.
		case candle := <-m.updateSignals:
			m.handleUpdateCandle(&candle)
		default:
			// fallthrough
		}
	}
}
