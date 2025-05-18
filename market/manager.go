package market

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
	// workerBufferSize is the default buffer size for workers.
	workerBufferSize = 4
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
	// averageVolumeRange is the minimum range for average volume calculations.
	averageVolumeRange = 30
	// fiveMinutesInSeconds is five minutes in seconds.
	fiveMinutesInSeconds = 300
	// maxRetries is the maximum number of retries allowed.
	maxRetries = 3
)

// ManagerConfig represents the market manager configuration.
type ManagerConfig struct {
	// Markets represents the collection of ids of the markets to manage.
	Markets []string
	// Backtest is the backtesting flag.
	Backtest bool
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(name string, sub chan shared.Candlestick)
	// RelayMarketUpdate relays the provided market update to the price action
	// manager for processing.
	RelayMarketUpdate func(candle shared.Candlestick)
	// CatchUp signals a catchup process for a market.
	CatchUp func(signal shared.CatchUpSignal)
	// SignalLevel relays the provided  level signal for  processing.
	SignalLevel func(signal shared.LevelSignal)
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager manages the lifecycle processes of all tracked markets.
type Manager struct {
	cfg                   *ManagerConfig
	markets               map[string]*Market
	marketsMtx            sync.RWMutex
	updateSignals         chan shared.Candlestick
	caughtUpSignals       chan shared.CaughtUpSignal
	priceDataRequests     chan shared.PriceDataRequest
	averageVolumeRequests chan shared.AverageVolumeRequest
	vwapDataRequests      chan shared.VWAPDataRequest
	vwapRequests          chan shared.VWAPRequest
	workers               map[string]chan struct{}
	requestWorkers        chan struct{}
}

// NewManager initializes a new market manager.
func NewManager(cfg *ManagerConfig, now time.Time) (*Manager, error) {
	// initialize managed markets.
	markets := make(map[string]*Market, 0)
	workers := make(map[string]chan struct{})
	for idx := range cfg.Markets {
		workers[cfg.Markets[idx]] = make(chan struct{}, workerBufferSize)

		mCfg := &MarketConfig{
			Market:            cfg.Markets[idx],
			SignalLevel:       cfg.SignalLevel,
			RelayMarketUpdate: cfg.RelayMarketUpdate,
			JobScheduler:      cfg.JobScheduler,
		}
		market, err := NewMarket(mCfg, now)
		if err != nil {
			return nil, fmt.Errorf("creating market: %w", err)
		}

		markets[cfg.Markets[idx]] = market
	}

	return &Manager{
		cfg:                   cfg,
		markets:               markets,
		updateSignals:         make(chan shared.Candlestick, bufferSize),
		priceDataRequests:     make(chan shared.PriceDataRequest, bufferSize),
		averageVolumeRequests: make(chan shared.AverageVolumeRequest, bufferSize),
		caughtUpSignals:       make(chan shared.CaughtUpSignal, bufferSize),
		vwapDataRequests:      make(chan shared.VWAPDataRequest, bufferSize),
		vwapRequests:          make(chan shared.VWAPRequest, bufferSize),
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
func (m *Manager) SendPriceDataRequest(request shared.PriceDataRequest) {
	select {
	case m.priceDataRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("price data requests channel at capacity: %d/%d",
			len(m.priceDataRequests), bufferSize)
	}
}

// SendVWAPDataRequest relays the provided vwap request for processing.
func (m *Manager) SendVWAPDataRequest(request shared.VWAPDataRequest) {
	select {
	case m.vwapDataRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("vwap data requests channel at capacity: %d/%d",
			len(m.vwapDataRequests), bufferSize)
	}
}

// SendVWAPRequest relays the provided vwap request for processing.
func (m *Manager) SendVWAPRequest(request shared.VWAPRequest) {
	select {
	case m.vwapRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("current vwap requests channel at capacity: %d/%d",
			len(m.vwapRequests), bufferSize)
	}
}

// SendAverageVolumeRequest relays the provided average volume request for processing.
func (m *Manager) SendAverageVolumeRequest(request shared.AverageVolumeRequest) {
	select {
	case m.averageVolumeRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("average volume requests channel at capacity: %d/%d",
			len(m.averageVolumeRequests), bufferSize)
	}
}

// FetchCaughtUpState returns the caught up statis of the provided market.
func (m *Manager) FetchCaughtUpState(market string) (bool, error) {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[market]
	m.marketsMtx.RUnlock()

	if !ok {
		return false, fmt.Errorf("no market found with name %s", market)
	}

	return mkt.CaughtUp(), nil
}

// handleUpdateSignal processes the provided market update candle.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) error {
	defer func() {
		candle.Status <- shared.Processed
	}()

	m.marketsMtx.RLock()
	mkt, ok := m.markets[candle.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s for update", candle.Market)
	}

	err := mkt.Update(candle)
	if err != nil {
		return fmt.Errorf("updating %s market: %v", candle.Market, err)
	}

	return nil
}

// handleCaughtUpSignal processes the provided caught up signal.
func (m *Manager) handleCaughtUpSignal(signal *shared.CaughtUpSignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	m.marketsMtx.RLock()
	mkt, ok := m.markets[signal.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s for update", signal.Market)
	}

	mkt.SetCaughtUpStatus(true)

	return nil
}

// handleAverageVolumeRequest processes the provided average volume request.
func (m *Manager) handleAverageVolumeRequest(req *shared.AverageVolumeRequest) error {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s", req.Market)
	}

	candleSnapshot, ok := mkt.candleSnapshots[req.Timeframe]
	if !ok {
		return fmt.Errorf("no candle snapshot found for market %s with timeframe %s", req.Market, req.Timeframe)
	}

	avgVolume := candleSnapshot.AverageVolumeN(averageVolumeRange)
	req.Response <- avgVolume

	return nil
}

// handlePriceDataRequest process the provided price data request.
func (m *Manager) handlePriceDataRequest(req *shared.PriceDataRequest) error {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s", req.Market)
	}

	if !mkt.CaughtUp() {
		return fmt.Errorf("%s is not caught up to current market data", req.Market)
	}

	candleSnapshot, ok := mkt.candleSnapshots[req.Timeframe]
	if !ok {
		return fmt.Errorf("no candle snapshot for market %s found for timeframe %s",
			req.Market, req.Timeframe)
	}

	data := candleSnapshot.LastN(int32(req.N))
	req.Response <- data

	return nil
}

// handleVWAPDataRequest processes the provided vwap data request.
func (m *Manager) handleVWAPDataRequest(req *shared.VWAPDataRequest) error {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s", req.Market)
	}

	if !mkt.CaughtUp() {
		return fmt.Errorf("%s is not caught up to current market data", req.Market)
	}

	vwapSnapshot, ok := mkt.vwapSnapshots[req.Timeframe]
	if !ok {
		return fmt.Errorf("no vwap snapshot for market %s found for timeframe %s",
			req.Market, req.Timeframe)
	}

	data := vwapSnapshot.LastN(shared.VWAPDataPayloadSize)
	req.Response <- data

	return nil
}

// handleVWAPRequest processes the provided current vwap request.
func (m *Manager) handleVWAPRequest(req *shared.VWAPRequest) error {
	m.marketsMtx.RLock()
	mkt, ok := m.markets[req.Market]
	m.marketsMtx.RUnlock()

	if !ok {
		return fmt.Errorf("no market found with name %s", req.Market)
	}

	if !mkt.CaughtUp() {
		return fmt.Errorf("%s is not caught up to current market data", req.Market)
	}

	vwapSnapshot, ok := mkt.vwapSnapshots[req.Timeframe]
	if !ok {
		return fmt.Errorf("no vwap snapshot for market %s found for timeframe %s",
			req.Market, req.Timeframe)
	}

	vwap := vwapSnapshot.At(req.At)
	req.Response <- vwap

	return nil
}

// catchup signals a catch up for all tracked markets.
func (m *Manager) catchUp() error {
	m.marketsMtx.RLock()
	defer m.marketsMtx.RUnlock()

	for idx := range m.markets {
		market := m.markets[idx]

		start, err := market.sessionSnapshot.FetchLastSessionOpen()
		if err != nil {
			return fmt.Errorf("fetching last session open: %v", err)
		}

		signal := shared.NewCatchUpSignal(market.cfg.Market, shared.FiveMinute, start)
		m.cfg.CatchUp(signal)
	}

	return nil
}

// Run manages the lifecycle processes of the position manager.
func (m *Manager) Run(ctx context.Context) {
	const marketManager = "marketmanager"
	m.cfg.Subscribe(marketManager, m.updateSignals)

	if !m.cfg.Backtest {
		// Catch up only in live execution environments.
		err := m.catchUp()
		if err != nil {
			m.cfg.Logger.Error().Err(err).Send()
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-m.updateSignals:
			// use the dedicated market worker to handle the update signal.
			m.workers[candle.Market] <- struct{}{}
			go func(candle shared.Candlestick) {
				err := m.handleUpdateCandle(&candle)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.workers[candle.Market]
			}(candle)
		case signal := <-m.caughtUpSignals:
			// use the dedicated market worker to handle the caught up signal.
			m.workers[signal.Market] <- struct{}{}
			go func(signal shared.CaughtUpSignal) {
				err := m.handleCaughtUpSignal(&signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.workers[signal.Market]
			}(signal)
		case req := <-m.priceDataRequests:
			// handle price data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req shared.PriceDataRequest) {
				err := m.handlePriceDataRequest(&req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.requestWorkers
			}(req)
		case req := <-m.vwapDataRequests:
			// handle vwap data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req shared.VWAPDataRequest) {
				err := m.handleVWAPDataRequest(&req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.requestWorkers
			}(req)
		case req := <-m.vwapRequests:
			// handle vwap requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req shared.VWAPRequest) {
				err := m.handleVWAPRequest(&req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.requestWorkers
			}(req)
		case req := <-m.averageVolumeRequests:
			// handle average volume data requests concurrently.
			m.requestWorkers <- struct{}{}
			go func(req shared.AverageVolumeRequest) {
				err := m.handleAverageVolumeRequest(&req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
					return
				}
				<-m.requestWorkers
			}(req)
		default:
			// fallthrough
		}
	}
}
