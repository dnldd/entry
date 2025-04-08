package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
	// minPriceDataRange is the minimum number of candles sent for a price data range request.
	minPriceDataRange = 5
)

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe shared.Timeframe
	Start     time.Time
}

// LevelSignal represents a level signal to outline a price level.
type LevelSignal struct {
	Market string
	Price  float64
}

// PriceDataRequest represents a price data request to fetch price data for a time range.
type PriceDataRequest struct {
	Market   string
	Response *chan []*shared.Candlestick
}

// ManagerConfig represents the market manager configuration.
type ManagerConfig struct {
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(sub *chan shared.Candlestick)
	// CatchUp signals a catchup process for a market.
	CatchUp func(signal CatchUpSignal)
	// SignalLevel relays the provided  level signal for  processing.
	SignalLevel func(signal LevelSignal)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager manages the lifecycle processes of all tracked markets.
type Manager struct {
	cfg               *ManagerConfig
	markets           map[string]*Market
	marketsMtx        sync.RWMutex
	updateSignals     chan shared.Candlestick
	priceDataRequests chan *PriceDataRequest
	workers           map[string]chan struct{}
	requestWorkers    chan struct{}
}

// NewManager initializes a new market manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	// initialize managed markets.
	markets := make(map[string]*Market, 0)
	workers := make(map[string]chan struct{})
	for idx := range cfg.MarketIDs {
		workers[cfg.MarketIDs[idx]] = make(chan struct{})

		mCfg := &MarketConfig{
			Market:      cfg.MarketIDs[idx],
			SignalLevel: cfg.SignalLevel,
		}
		market, err := NewMarket(mCfg)
		if err != nil {
			return nil, fmt.Errorf("creating market: %w", err)
		}

		markets[cfg.MarketIDs[idx]] = market
	}

	return &Manager{
		cfg:               cfg,
		markets:           markets,
		updateSignals:     make(chan shared.Candlestick, bufferSize),
		priceDataRequests: make(chan *PriceDataRequest, bufferSize),
		workers:           workers,
		requestWorkers:    make(chan struct{}, maxWorkers),
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

// SendPriceDataRequest relays the provided price data request for processing.
func (m *Manager) SendPriceDataRequest(request *PriceDataRequest) {
	select {
	case m.priceDataRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("price data requests channel at capacity: %d/%d",
			len(m.priceDataRequests), bufferSize)
	}
}

// handleUpdateSignal processes the provided market update candle.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) {
	m.marketsMtx.RLock()
	market, ok := m.markets[candle.Market]
	m.marketsMtx.RUnlock()

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

// handlePriceDateRequest process the requested price data.
func (m *Manager) handlePriceDataRequest(req *PriceDataRequest) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s", req.Market)
		return
	}

	data := mkt.candleSnapshot.LastN(minPriceDataRange)

	*req.Response <- data
}

// catchup signals a catch up for all tracked markets.
func (m *Manager) catchUp() {
	m.marketsMtx.RLock()
	defer m.marketsMtx.RUnlock()

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
	m.cfg.Subscribe(&m.updateSignals)
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
		case req := <-m.priceDataRequests:
			// handle price data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req *PriceDataRequest) {
				m.handlePriceDataRequest(req)
				<-m.requestWorkers
			}(req)
		default:
			// fallthrough
		}
	}
}
