package market

import (
	"context"
	"fmt"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
)

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe shared.Timeframe
	Start     time.Time
}

// ManagerConfig represents the market manager configuration.
type ManagerConfig struct {
	// MarketIDs represetns the collection of ids of the markets to manage.
	MarketIDs []string
	// CatchUp signals a catchup process for a market.
	CatchUp func(signal CatchUpSignal)
	// SignalSupport relays the provided support.
	SignalSupport func(price float64)
	// SignalResistance relays the provided resistance.
	SignalResistance func(price float64)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager manages the lifecycle processes of all tracked markets.
type Manager struct {
	cfg           *ManagerConfig
	markets       map[string]*Market
	updateSignals chan shared.Candlestick
	workers       map[string]chan struct{}
}

// NewManager initializes a new market manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	// initialize managed markets.
	markets := make(map[string]*Market, 0)
	workers := make(map[string]chan struct{})
	for idx := range cfg.MarketIDs {
		workers[cfg.MarketIDs[idx]] = make(chan struct{})

		mCfg := &MarketConfig{
			Market:           cfg.MarketIDs[idx],
			SignalSupport:    cfg.SignalSupport,
			SignalResistance: cfg.SignalResistance,
		}
		market, err := NewMarket(mCfg)
		if err != nil {
			return nil, fmt.Errorf("creating market: %w", err)
		}

		markets[cfg.MarketIDs[idx]] = market
	}

	return &Manager{
		cfg:           cfg,
		markets:       markets,
		updateSignals: make(chan shared.Candlestick, bufferSize),
		workers:       workers,
	}, nil
}

// SendMarketUpdate relays the provided candlestick for processing.
func (m *Manager) SendMarketUpdate(candle shared.Candlestick) {
	select {
	case m.updateSignals <- candle:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market update channel at capacity: %d/%d",
			len(m.updateSignals), bufferSize)
	}
}

// handleUpdateSignal processes the provided market update candle.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) {
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
func (m *Manager) catchUp() {
	for _, v := range m.markets {
		market := *v

		start, err := market.sessionSnapshot.FetchLastSessionOpen()
		if err != nil {
			m.cfg.Logger.Error().Msgf("fetching last session open: %v", err)
		}

		signal := CatchUpSignal{
			Market:    market.cfg.Market,
			Timeframe: shared.FiveMinute,
			Start:     start,
		}

		m.cfg.CatchUp(signal)
	}
}

// Run manages the lifecycle processes of the position manager.
func (m *Manager) Run(ctx context.Context) {
	m.catchUp()

	for {
		select {
		case candle := <-m.updateSignals:
			// use the dedicated market worker to handle the update signal.
			m.workers[candle.Market] <- struct{}{}
			go func(candle *shared.Candlestick) {
				m.handleUpdateCandle(candle)
				<-m.workers[candle.Market]
			}(&candle)
		default:
			// fallthrough
		}
	}
}
