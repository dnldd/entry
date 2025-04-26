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
)

// ManagerConfig represents the market manager configuration.
type ManagerConfig struct {
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(sub chan shared.Candlestick)
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
	averageVolume         map[string]shared.AverageVolumeEntry
	averageVolumeMtx      sync.RWMutex
	updateSignals         chan shared.Candlestick
	caughtUpSignals       chan shared.CaughtUpSignal
	priceDataRequests     chan shared.PriceDataRequest
	averageVolumeRequests chan shared.AverageVolumeRequest
	workers               map[string]chan struct{}
	requestWorkers        chan struct{}
}

// NewManager initializes a new market manager.
func NewManager(cfg *ManagerConfig, now time.Time) (*Manager, error) {
	// initialize managed markets.
	markets := make(map[string]*Market, 0)
	workers := make(map[string]chan struct{})
	for idx := range cfg.MarketIDs {
		workers[cfg.MarketIDs[idx]] = make(chan struct{}, workerBufferSize)

		mCfg := &MarketConfig{
			Market:       cfg.MarketIDs[idx],
			SignalLevel:  cfg.SignalLevel,
			JobScheduler: cfg.JobScheduler,
		}
		market, err := NewMarket(mCfg, now)
		if err != nil {
			return nil, fmt.Errorf("creating market: %w", err)
		}

		markets[cfg.MarketIDs[idx]] = market
	}

	return &Manager{
		cfg:                   cfg,
		markets:               markets,
		updateSignals:         make(chan shared.Candlestick, bufferSize),
		priceDataRequests:     make(chan shared.PriceDataRequest, bufferSize),
		averageVolumeRequests: make(chan shared.AverageVolumeRequest, bufferSize),
		caughtUpSignals:       make(chan shared.CaughtUpSignal, bufferSize),
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
func (m *Manager) SendPriceDataRequest(request shared.PriceDataRequest) {
	select {
	case m.priceDataRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("price data requests channel at capacity: %d/%d",
			len(m.priceDataRequests), bufferSize)
	}
}

// SendAverageVolumeRequest relays the provided average volume request for processing.
func (m *Manager) SendAverageVolumeRequest(request shared.AverageVolumeRequest) {
	select {
	case m.averageVolumeRequests <- request:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("average volume requests channel at capacity: %d/%d",
			len(m.averageVolume), bufferSize)
	}
}

// handleUpdateSignal processes the provided market update candle.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) error {
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
	m.averageVolumeMtx.RLock()
	avgVolume, ok := m.averageVolume[req.Market]
	m.averageVolumeMtx.RUnlock()

	now, _, err := shared.NewYorkTime()
	if err != nil {
		return fmt.Errorf("fetching new york time: %v", err)
	}

	if ok && now.Unix()-avgVolume.CreatedAt < fiveMinutesInSeconds {
		req.Response <- avgVolume.Average

		return nil
	}

	if now.Unix()-avgVolume.CreatedAt > fiveMinutesInSeconds {
		// Generate a new average volume entry for the market if it is older than five minutes.
		m.marketsMtx.RLock()
		mkt, ok := m.markets[req.Market]
		m.marketsMtx.RUnlock()

		if !ok {
			return fmt.Errorf("no market found with name %s", req.Market)
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

	req.Response <- avgVolume.Average

	return nil
}

// handlePriceDateRequest process the requested price data.
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

	data := mkt.candleSnapshot.LastN(shared.PriceDataPayloadSize)

	req.Response <- data

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

		signal := shared.CatchUpSignal{
			Market:    market.cfg.Market,
			Timeframe: shared.FiveMinute,
			Start:     start,
			Done:      make(chan struct{}),
		}

		m.cfg.CatchUp(signal)
	}

	return nil
}

// Run manages the lifecycle processes of the position manager.
func (m *Manager) Run(ctx context.Context) {
	m.cfg.Subscribe(m.updateSignals)
	err := m.catchUp()
	if err != nil {
		m.cfg.Logger.Error().Err(err).Send()
		return
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
