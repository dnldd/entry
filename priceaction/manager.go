package priceaction

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
	// workerBufferSize is the default buffer size for workers.
	workerBufferSize = 4
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
	// candleMetadataSize is the required elements for fetching candle metadata.
	candleMetadataSize = 4
)

// ManagerConfig represents the price action manager configuration.
type ManagerConfig struct {
	// Markets represents the collection of ids of the markets to manage.
	Markets []string
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(name string, sub chan shared.Candlestick)
	// RequestPriceData sends a price data request.
	RequestPriceData func(request shared.PriceDataRequest)
	// RequestVWAPData relays the provided vwap request for processing.
	RequestVWAPData func(request shared.VWAPDataRequest)
	// RequestVWAP relays the provided vwap request for processing.
	RequestVWAP func(request shared.VWAPRequest)
	// SignalLevelReaction relays a level reaction for processing.
	SignalLevelReaction func(signal shared.LevelReaction)
	// SignalVWAPReaction relays a vwap reaction for processing.
	SignalVWAPReaction func(signal shared.VWAPReaction)
	// FetchCaughtUpState returns the caught up statis of the provided market.
	FetchCaughtUpState func(market string) (bool, error)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager represents the price action manager.
type Manager struct {
	cfg            *ManagerConfig
	markets        map[string]*Market
	levelSignals   chan shared.LevelSignal
	updateSignals  chan shared.Candlestick
	metaSignals    chan shared.CandleMetadataRequest
	workers        map[string]chan struct{}
	requestWorkers chan struct{}
}

// NewManager initializes a new price action manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	markets := make(map[string]*Market)
	workers := make(map[string]chan struct{})
	for idx := range cfg.Markets {
		market := cfg.Markets[idx]

		workers[market] = make(chan struct{}, workerBufferSize)

		cfg := &MarketConfig{
			Market:             market,
			RequestVWAP:        cfg.RequestVWAP,
			RequestVWAPData:    cfg.RequestVWAPData,
			FetchCaughtUpState: cfg.FetchCaughtUpState,
			Logger:             cfg.Logger,
		}
		mkt, err := NewMarket(cfg)
		if err != nil {
			return nil, fmt.Errorf("creating %s market: %v", market, err)
		}

		markets[market] = mkt
	}
	return &Manager{
		cfg:            cfg,
		markets:        markets,
		levelSignals:   make(chan shared.LevelSignal, bufferSize),
		updateSignals:  make(chan shared.Candlestick, bufferSize),
		metaSignals:    make(chan shared.CandleMetadataRequest, bufferSize),
		requestWorkers: make(chan struct{}, maxWorkers),
		workers:        workers,
	}, nil
}

// SendLevel relays the provided level signal for processing.
func (m *Manager) SendLevelSignal(level shared.LevelSignal) {
	select {
	case m.levelSignals <- level:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("level channel at capacity: %d/%d",
			len(m.levelSignals), bufferSize)
	}
}

// SendLevel relays the provided market update for processing.
func (m *Manager) SendMarketUpdate(candle shared.Candlestick) {
	select {
	case m.updateSignals <- candle:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market update channel at capacity: %d/%d",
			len(m.updateSignals), bufferSize)
	}
}

// SendCandleMetadataRequest relays the provided candle metadata signal for processing.
func (m *Manager) SendCandleMetadataRequest(req shared.CandleMetadataRequest) {
	select {
	case m.metaSignals <- req:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("candle metadata request channel at capacity: %d/%d",
			len(m.metaSignals), bufferSize)
	}
}

// handleUpdateSignal processes the provided update signal.
func (m *Manager) handleUpdateSignal(candle *shared.Candlestick) error {
	defer func() {
		candle.Status <- shared.Processed
	}()

	mkt, ok := m.markets[candle.Market]
	if !ok {
		return fmt.Errorf("no market found with name: %s", candle.Market)
	}

	// Update price action concepts related to the market.
	mkt.Update(candle)
	if mkt.RequestingPriceData() {
		// Request price data and generate price reactions from them.
		req := shared.NewPriceDataRequest(mkt.cfg.Market)
		m.cfg.RequestPriceData(*req)
		var data []*shared.Candlestick
		select {
		case data = <-req.Response:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for price data response")
		}

		levelReactions, err := mkt.GenerateLevelReactions(data)
		if err != nil {
			return fmt.Errorf("generating level reactions: %v", err)
		}

		for idx := range levelReactions {
			levelReaction := levelReactions[idx]
			m.cfg.SignalLevelReaction(*levelReaction)
			select {
			case <-levelReaction.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out waiting for level reaction status")
			}
		}

		mkt.ResetPriceDataState()
	}

	if mkt.RequestingVWAPData() {
		// Request price data and vwap data and generate price reactions from them.
		priceReq := shared.NewPriceDataRequest(mkt.cfg.Market)
		m.cfg.RequestPriceData(*priceReq)
		var priceData []*shared.Candlestick
		select {
		case priceData = <-priceReq.Response:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for price data response")
		}

		vwapReq := shared.NewVWAPDataRequest(mkt.cfg.Market)
		m.cfg.RequestVWAPData(*vwapReq)
		var vwapData []*shared.VWAP
		select {
		case vwapData = <-vwapReq.Response:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for vwap data response")
		}

		vwapReaction, err := shared.NewVWAPReaction(mkt.cfg.Market, vwapData, priceData)
		if err != nil {
			return fmt.Errorf("creating vwap reaction: %v", err)
		}

		m.cfg.SignalVWAPReaction(*vwapReaction)
		select {
		case <-vwapReaction.Status:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for vwap reaction status")
		}
	}

	return nil
}

// handleLevelSignal processes the provided level signal.
func (m *Manager) handleLevelSignal(signal shared.LevelSignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	mkt, ok := m.markets[signal.Market]
	if !ok {
		return fmt.Errorf("no market found with name %s", signal.Market)
	}

	currentCandle := mkt.FetchCurrentCandle()
	if currentCandle == nil {
		return fmt.Errorf("no current candle available for market: %s", mkt.cfg.Market)
	}

	level := shared.NewLevel(signal.Market, signal.Price, currentCandle)
	mkt.levelSnapshot.Add(level)
	m.cfg.Logger.Info().Msgf("added new %s level @ %.2f for %s", level.Kind.String(), level.Price, level.Market)

	return nil
}

// handleCandleMetadataRequest processes the provided candle metadata request.
func (m *Manager) handleCandleMetadataRequest(req *shared.CandleMetadataRequest) error {
	mkt, ok := m.markets[req.Market]
	if !ok {
		return fmt.Errorf("no market found with name: %s", req.Market)
	}

	// Generate metadata for all candles in the range being evaluated.
	candles := mkt.candleSnapshot.LastN(candleMetadataSize + 1)
	metadataSet := make([]*shared.CandleMetadata, 0, candleMetadataSize)

	for idx := 1; idx < len(candles)-1; idx++ {
		currentCandle := candles[idx]
		previousCandle := candles[idx-1]

		kind := currentCandle.FetchKind()
		sentiment := currentCandle.FetchSentiment()
		momentum := shared.GenerateMomentum(currentCandle, previousCandle)
		isEngulfing := shared.IsEngulfing(currentCandle, previousCandle)

		meta := &shared.CandleMetadata{
			Kind:      kind,
			Sentiment: sentiment,
			Momentum:  momentum,
			Volume:    currentCandle.Volume,
			Engulfing: isEngulfing,
			High:      currentCandle.High,
			Low:       currentCandle.Low,
			Date:      currentCandle.Date,
		}

		metadataSet = append(metadataSet, meta)
	}

	select {
	case req.Response <- metadataSet:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for candle metadata response")
	}

	return nil
}

// Run manages the lifecycle processes of the price action manager.
func (m *Manager) Run(ctx context.Context) {
	const priceActionManager = "priceactionmanager"
	m.cfg.Subscribe(priceActionManager, m.updateSignals)

	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-m.levelSignals:
			m.workers[signal.Market] <- struct{}{}
			go func(signal shared.LevelSignal) {
				err := m.handleLevelSignal(signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers[signal.Market]
			}(signal)
		case candle := <-m.updateSignals:
			m.workers[candle.Market] <- struct{}{}
			go func(candle shared.Candlestick) {
				err := m.handleUpdateSignal(&candle)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers[candle.Market]
			}(candle)
		case req := <-m.metaSignals:
			m.requestWorkers <- struct{}{}
			go func(req shared.CandleMetadataRequest) {
				err := m.handleCandleMetadataRequest(&req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.requestWorkers
			}(req)

		default:
			// fallthrough
		}
	}
}
