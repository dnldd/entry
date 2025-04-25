package priceaction

import (
	"context"
	"fmt"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
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
	Subscribe func(sub chan shared.Candlestick)
	// RequestPriceData sends a price data request.
	RequestPriceData func(request shared.PriceDataRequest)
	// SignalLevelReaction relays a level reaction for processing.
	SignalLevelReaction func(signal shared.LevelReaction)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager represents the price action manager.
type Manager struct {
	cfg           *ManagerConfig
	markets       map[string]*Market
	levelSignals  chan shared.LevelSignal
	updateSignals chan shared.Candlestick
	metaSignals   chan shared.CandleMetadataRequest
	workers       chan struct{}
}

// NewManager initializes a new price action manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	markets := make(map[string]*Market)
	for idx := range cfg.Markets {
		mkt, err := NewMarket(cfg.Markets[idx])
		if err != nil {
			return nil, fmt.Errorf("creating %s market: %v", cfg.Markets[idx], err)
		}

		markets[cfg.Markets[idx]] = mkt
	}
	return &Manager{
		cfg:           cfg,
		markets:       markets,
		levelSignals:  make(chan shared.LevelSignal, bufferSize),
		updateSignals: make(chan shared.Candlestick, bufferSize),
		metaSignals:   make(chan shared.CandleMetadataRequest, bufferSize),
		workers:       make(chan struct{}, maxWorkers),
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

// SendLevel relays the provided level signal for processing.
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
	mkt, ok := m.markets[candle.Market]
	if !ok {
		return fmt.Errorf("no market found with name: %s", candle.Market)
	}

	// Update price action concepts related to the market.
	mkt.Update(candle)
	if mkt.RequestingPriceData() {
		// Request price data and generate price reactions from them.
		req := shared.PriceDataRequest{
			Market:   mkt.market,
			Response: make(chan []*shared.Candlestick),
		}

		m.cfg.RequestPriceData(req)
		data := <-req.Response

		reactions, err := mkt.GenerateLevelReactions(data)
		if err != nil {
			return fmt.Errorf("generating level reactions: %v", err)
		}

		for idx := range reactions {
			m.cfg.SignalLevelReaction(*reactions[idx])
		}

		mkt.ResetPriceDataState()
	}

	return nil
}

// handleLevelSignal processes the provided level signal.
func (m *Manager) handleLevelSignal(signal shared.LevelSignal) error {
	mkt, ok := m.markets[signal.Market]
	if !ok {
		return fmt.Errorf("no market found with name %s", signal.Market)
	}

	currentCandle := mkt.FetchCurrentCandle()
	if currentCandle == nil {
		return fmt.Errorf("no current candle available for market: %s", mkt.market)
	}

	level := shared.NewLevel(signal.Market, signal.Price, currentCandle)
	mkt.levelSnapshot.Add(level)

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

	req.Response <- metadataSet

	return nil
}

// Run manages the lifecycle processes of the price action manager.
func (m *Manager) Run(ctx context.Context) {
	m.cfg.Subscribe(m.updateSignals)

	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-m.levelSignals:
			m.workers <- struct{}{}
			go func(signal *shared.LevelSignal) {
				err := m.handleLevelSignal(*signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&signal)
		case candle := <-m.updateSignals:
			m.workers <- struct{}{}
			go func(candle *shared.Candlestick) {
				err := m.handleUpdateSignal(candle)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&candle)
		case req := <-m.metaSignals:
			m.workers <- struct{}{}
			go func(req *shared.CandleMetadataRequest) {
				err := m.handleCandleMetadataRequest(req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&req)

		default:
			// fallthrough
		}
	}
}
