package priceaction

import (
	"context"
	"sync"

	"github.com/dnldd/entry/market"
	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
)

// ManagerConfig represents the price action manager configuration.
type ManagerConfig struct {
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(sub *chan shared.Candlestick)
	// RequestPriceData sends a price data request.
	RequestPriceData func(request *market.PriceDataRequest)
	// SignalPriceLevelReactions relays price level reactions for processing.
	SignalPriceLevelReactions func(signal PriceLevelReactionsSignal)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager represents the price action manager.
type Manager struct {
	cfg           *ManagerConfig
	markets       map[string]*Market
	marketsMtx    sync.RWMutex
	levelSignals  chan market.LevelSignal
	updateSignals chan shared.Candlestick
	workers       chan struct{}
}

// NewManager initializes a new price action manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	return &Manager{
		cfg:           cfg,
		markets:       make(map[string]*Market),
		levelSignals:  make(chan market.LevelSignal, bufferSize),
		updateSignals: make(chan shared.Candlestick),
		workers:       make(chan struct{}, maxWorkers),
	}, nil
}

// SendLevel relays the provided level signal for processing.
func (m *Manager) SendLevelSignal(level market.LevelSignal) {
	select {
	case m.levelSignals <- level:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("level channel at capacity: %d/%d",
			len(m.levelSignals), bufferSize)
	}
}

// handleUpdateSignal processes the provided update signal.
func (m *Manager) handleUpdateSignal(candle *shared.Candlestick) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[candle.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		// Create and track a new market if it does not exist yet.
		var err error
		mkt, err = NewMarket(candle.Market)
		if err != nil {
			m.cfg.Logger.Error().Msgf("creating %s market: %v", candle.Market, err)
			return
		}

		m.marketsMtx.Lock()
		m.markets[candle.Market] = mkt
		m.marketsMtx.Unlock()
	}

	// Update price action concepts related to the market.
	mkt.Update(candle)
	if mkt.RequestingPriceData() {
		// Request price data and generate price reactions from them.
		resp := make(chan []*shared.Candlestick)
		req := &market.PriceDataRequest{
			Market:   mkt.market,
			Response: &resp,
		}

		m.cfg.RequestPriceData(req)
		data := <-resp

		reactions := mkt.GeneratePriceReaction(data)
		signal := PriceLevelReactionsSignal{
			Market:    mkt.market,
			Reactions: reactions,
		}
		m.cfg.SignalPriceLevelReactions(signal)

		mkt.ResetPriceDataState()
	}

}

// handleLevelSignal processes the provided level signal.
func (m *Manager) handleLevelSignal(signal market.LevelSignal) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[signal.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s", signal.Market)
		return
	}

	currentCandle := mkt.FetchCurrentCandle()
	if currentCandle == nil {
		m.cfg.Logger.Error().Msgf("no current candle available, skipping level")
		return
	}

	level := NewLevel(signal.Market, signal.Price, currentCandle)
	mkt.levelSnapshot.Add(level)
}

// Run manages the lifecycle processes of the price action manager.
func (m *Manager) Run(ctx context.Context) {
	m.cfg.Subscribe(&m.updateSignals)

	for {
		select {
		case signal := <-m.levelSignals:
			m.workers <- struct{}{}
			go func(level *market.LevelSignal) {
				m.handleLevelSignal(signal)
				<-m.workers
			}(&signal)
		case candle := <-m.updateSignals:
			m.workers <- struct{}{}
			go func(candle *shared.Candlestick) {
				m.handleUpdateSignal(candle)
				<-m.workers
			}(&candle)
		default:
			// fallthrough
		}
	}
}
