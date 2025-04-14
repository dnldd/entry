package market

import (
	"context"
	"fmt"
	"sync"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
	// minPriceDataRange is the minimum number of candles sent for a price data range request.
	minPriceDataRange = 4
	// averageVolumeRange is the minimum range for average volume calculations.
	averageVolumeRange = 30
	// fiveMinutesInSeconds is five minutes in seconds.
	fiveMinutesInSeconds = 300
)

// ManagerConfig represents the market manager configuration.
type ManagerConfig struct {
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(sub *chan shared.Candlestick)
	// CatchUp signals a catchup process for a market.
	CatchUp func(signal shared.CatchUpSignal)
	// SignalLevel relays the provided  level signal for  processing.
	SignalLevel func(signal shared.LevelSignal)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager manages the lifecycle processes of all tracked markets.
type Manager struct {
	cfg                   *ManagerConfig
	markets               map[string]*Market
	marketsMtx            sync.RWMutex
	averageVolume         map[string]shared.AverageVolumeEntry
	averageVolumeMtx      sync.RWMutex
	updateSignals         chan shared.Candlestick
	caughtUpSignals       chan shared.CaughtUpSignal
	priceDataRequests     chan *shared.PriceDataRequest
	averageVolumeRequests chan *shared.AverageVolumeRequest
	workers               map[string]chan struct{}
	requestWorkers        chan struct{}
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
		cfg:                   cfg,
		markets:               markets,
		updateSignals:         make(chan shared.Candlestick, bufferSize),
		priceDataRequests:     make(chan *shared.PriceDataRequest, bufferSize),
		averageVolumeRequests: make(chan *shared.AverageVolumeRequest, bufferSize),
		averageVolume:         make(map[string]shared.AverageVolumeEntry),
		workers:               workers,
		requestWorkers:        make(chan struct{}, maxWorkers),
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

// SendCaughtUpSignal relays the provided caught up signal for processing.
func (m *Manager) SendCaughtUpSignal(signal shared.CaughtUpSignal) {
	select {
	case m.caughtUpSignals <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("caught up signal update channel at capacity: %d/%d",
			len(m.caughtUpSignals), bufferSize)
	}
}

// SendPriceDataRequest relays the provided price data request for processing.
func (m *Manager) SendPriceDataRequest(request *shared.PriceDataRequest) {
	select {
	case m.priceDataRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("price data requests channel at capacity: %d/%d",
			len(m.priceDataRequests), bufferSize)
	}
}

// SendAverageVolumeRequest relays the provided average volume request for processing.
func (m *Manager) SendAverageVolumeRequest(request *shared.AverageVolumeRequest) {
	select {
	case m.averageVolumeRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("average volume requests channel at capacity: %d/%d",
			len(m.averageVolume), bufferSize)
	}
}

// handleUpdateSignal processes the provided market update candle.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[candle.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s for update", candle.Market)
		return
	}

	err := mkt.Update(candle)
	if err != nil {
		m.cfg.Logger.Error().Msgf("updating %s market: %v", candle.Market, err)
		return
	}
}

// handleCaughtUpSignal processes the provided caught up signal.
func (m *Manager) handleCaughtUpSignal(signal *shared.CaughtUpSignal) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[signal.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s for update", signal.Market)
		return
	}

	mkt.SetCaughtUpStatus(true)
}

// handleAverageVolumeRequest processes the provided average volume request.
func (m *Manager) handleAverageVolumeRequest(req *shared.AverageVolumeRequest) {
	m.averageVolumeMtx.RLock()
	avgVolume, ok := m.averageVolume[req.Market]
	m.averageVolumeMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no average volume for %s market", req.Market)
		return
	}

	now, _, err := shared.NewYorkTime()
	if err != nil {
		m.cfg.Logger.Error().Msgf("fetching new york time: %v", err)
	}

	if now.Unix()-avgVolume.CreatedAt > fiveMinutesInSeconds {
		// Generate a new average volume entry for the market if it is older than five minutes.
		m.marketsMtx.RLock()
		mkt := m.markets[req.Market]
		m.marketsMtx.RUnlock()

		if !ok {
			m.cfg.Logger.Error().Msgf("no market found with name %s", req.Market)
			return
		}

		avg := mkt.candleSnapshot.AverageVolumeN(averageVolumeRange)

		avgVolume = shared.AverageVolumeEntry{
			Average:   avg,
			CreatedAt: now.Unix(),
		}

		m.averageVolumeMtx.Lock()
		m.averageVolume[req.Market] = avgVolume
		m.averageVolumeMtx.Unlock()
	}

	*req.Response <- avgVolume.Average
}

// handlePriceDateRequest process the requested price data.
func (m *Manager) handlePriceDataRequest(req *shared.PriceDataRequest) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		m.cfg.Logger.Error().Msgf("no market found with name %s", req.Market)
		return
	}

	if !mkt.CaughtUp() {
		m.cfg.Logger.Error().Msgf("%s is not caught up to current market data", req.Market)
		return
	}

	data := mkt.candleSnapshot.LastN(minPriceDataRange)

	*req.Response <- data
}

// catchup signals a catch up for all tracked markets.
func (m *Manager) catchUp() {
	m.marketsMtx.RLock()
	defer m.marketsMtx.RUnlock()

	for idx := range m.markets {
		market := m.markets[idx]

		start, err := market.sessionSnapshot.FetchLastSessionOpen()
		if err != nil {
			m.cfg.Logger.Error().Msgf("fetching last session open: %v", err)
		}

		signal := shared.CatchUpSignal{
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
		case <-ctx.Done():
			return
		case candle := <-m.updateSignals:
			// use the dedicated market worker to handle the update signal.
			m.workers[candle.Market] <- struct{}{}
			go func(candle *shared.Candlestick) {
				m.handleUpdateCandle(candle)
				<-m.workers[candle.Market]
			}(&candle)
		case candle := <-m.caughtUpSignals:
			// use the dedicated market worker to handle the caught up signal.
			m.workers[candle.Market] <- struct{}{}
			go func(signal *shared.CaughtUpSignal) {
				m.handleCaughtUpSignal(signal)
				<-m.workers[candle.Market]
			}(&candle)
		case req := <-m.priceDataRequests:
			// handle price data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req *shared.PriceDataRequest) {
				m.handlePriceDataRequest(req)
				<-m.requestWorkers
			}(req)
		case req := <-m.averageVolumeRequests:
			// handle average volume data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req *shared.AverageVolumeRequest) {
				m.handleAverageVolumeRequest(req)
				<-m.requestWorkers
			}(req)
		default:
			// fallthrough
		}
	}
}
