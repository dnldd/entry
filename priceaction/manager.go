package priceaction

import (
	"context"
	"errors"
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
	// SignalReactionAtLevel relays a reaction at a level for processing.
	SignalReactionAtLevel func(signal shared.ReactionAtLevel)
	// SignalVWAPReaction relays a vwap reaction for processing.
	SignalReactionAtVWAP func(signal shared.ReactionAtVWAP)
	// SignalReactionAtImbalance relays an imbalance reaction for processing.
	SignalReactionAtImbalance func(signal shared.ReactionAtImbalance)
	// FetchCaughtUpState returns the caught up statis of the provided market.
	FetchCaughtUpState func(market string) (bool, error)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Validate asserts the config sane inputs.
func (cfg *ManagerConfig) Validate() error {
	var errs error

	if len(cfg.Markets) == 0 {
		errs = errors.Join(errs, fmt.Errorf("no markets provided for position manager"))
	}
	if cfg.Subscribe == nil {
		errs = errors.Join(errs, fmt.Errorf("subscribe function cannot be nil"))
	}
	if cfg.RequestPriceData == nil {
		errs = errors.Join(errs, fmt.Errorf("request price data function cannot be nil"))
	}
	if cfg.RequestVWAPData == nil {
		errs = errors.Join(errs, fmt.Errorf("request vwap data function cannot be nil"))
	}
	if cfg.SignalReactionAtLevel == nil {
		errs = errors.Join(errs, fmt.Errorf("signal reaction at level function cannot be nil"))
	}
	if cfg.SignalReactionAtVWAP == nil {
		errs = errors.Join(errs, fmt.Errorf("signal reaction at vwap function cannot be nil"))
	}
	if cfg.SignalReactionAtImbalance == nil {
		errs = errors.Join(errs, fmt.Errorf("signal reaction at imbalance function cannot be nil"))
	}
	if cfg.FetchCaughtUpState == nil {
		errs = errors.Join(errs, fmt.Errorf("fetch caught up state function cannot be nil"))
	}
	if cfg.Logger == nil {
		errs = errors.Join(errs, fmt.Errorf("logger cannot be nil"))
	}

	return errs
}

// Manager represents the price action manager.
type Manager struct {
	cfg              *ManagerConfig
	markets          map[string]*Market
	levelSignals     chan shared.LevelSignal
	imbalanceSignals chan shared.ImbalanceSignal
	updateSignals    chan shared.Candlestick
	metaSignals      chan shared.CandleMetadataRequest
	workers          map[string]chan struct{}
	requestWorkers   chan struct{}
}

// NewManager initializes a new price action manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("validating price action manager config: %v", err)
	}

	markets := make(map[string]*Market)
	workers := make(map[string]chan struct{})
	for idx := range cfg.Markets {
		market := cfg.Markets[idx]

		workers[market] = make(chan struct{}, workerBufferSize)

		cfg := &MarketConfig{
			Market:             market,
			RequestVWAPData:    cfg.RequestVWAPData,
			RequestVWAP:        cfg.RequestVWAP,
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
		cfg:              cfg,
		markets:          markets,
		levelSignals:     make(chan shared.LevelSignal, bufferSize),
		imbalanceSignals: make(chan shared.ImbalanceSignal, bufferSize),
		updateSignals:    make(chan shared.Candlestick, bufferSize),
		metaSignals:      make(chan shared.CandleMetadataRequest, bufferSize),
		requestWorkers:   make(chan struct{}, maxWorkers),
		workers:          workers,
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

// SendImbalance relays the provided imbalance signal for processing.
func (m *Manager) SendImbalanceSignal(imbalance shared.ImbalanceSignal) {
	select {
	case m.imbalanceSignals <- imbalance:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("imbalance channel at capacity: %d/%d",
			len(m.imbalanceSignals), bufferSize)
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

// evaluateReactionAtLevelSignal determines whether a reaction at level signal should be generated for
// the provided market.
func (m *Manager) evaluateReactionAtLevelSignal(mkt *Market, timeframe shared.Timeframe) error {
	if !mkt.RequestingPriceData() {
		// Do nothing.
		return nil
	}

	// Request price data and generate price reactions from them.
	req := shared.NewPriceDataRequest(mkt.cfg.Market, timeframe, shared.PriceDataPayloadSize)
	m.cfg.RequestPriceData(*req)
	var data []*shared.Candlestick
	select {
	case data = <-req.Response:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for price data response")
	}

	reactions, err := mkt.GenerateReactionsAtTaggedLevels(data)
	if err != nil {
		return fmt.Errorf("generating level reactions: %v", err)
	}

	for idx := range reactions {
		reaction := reactions[idx]
		m.cfg.SignalReactionAtLevel(*reaction)
		select {
		case <-reaction.Status:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for reaction at level status")
		}
	}

	mkt.ResetPriceDataState()

	return nil
}

// evaluateReactionAtImbalanceSignal determines whether a reaction at imbalance signal should be
// generated for the provided market.
func (m *Manager) evaluateReactionAtImbalanceSignal(mkt *Market, timeframe shared.Timeframe) error {
	if !mkt.RequestingImbalanceData() {
		// Do nothing.
		return nil
	}

	// Request price data and generate price reactions from them.
	req := shared.NewPriceDataRequest(mkt.cfg.Market, timeframe, shared.PriceDataPayloadSize)
	m.cfg.RequestPriceData(*req)
	var data []*shared.Candlestick
	select {
	case data = <-req.Response:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for price data response")
	}

	reactions, err := mkt.GenerateReactionsAtTaggedImbalances(data)
	if err != nil {
		return fmt.Errorf("generating level reactions: %v", err)
	}

	for idx := range reactions {
		reaction := reactions[idx]
		m.cfg.SignalReactionAtImbalance(*reaction)
		select {
		case <-reaction.Status:
		case <-time.After(shared.TimeoutDuration):
			return fmt.Errorf("timed out waiting for reaction at imbalance status")
		}
	}

	mkt.ResetImbalanceDataState()

	return nil
}

// evaluateReactionAtVWAPSignal determines whether a reaction at vwap signal should be generated for
// the provided market.
func (m *Manager) evaluateReactionAtVWAPSignal(mkt *Market, timeframe shared.Timeframe) error {
	if !mkt.RequestingVWAPData() {
		// Do nothing.
		return nil
	}

	// Request price data and vwap data and generate price reactions from them.
	priceReq := shared.NewPriceDataRequest(mkt.cfg.Market, timeframe, shared.PriceDataPayloadSize)
	m.cfg.RequestPriceData(*priceReq)
	var priceData []*shared.Candlestick
	select {
	case priceData = <-priceReq.Response:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for price data response")
	}

	vwapReq := shared.NewVWAPDataRequest(mkt.cfg.Market, timeframe)
	m.cfg.RequestVWAPData(*vwapReq)
	var vwapData []*shared.VWAP
	select {
	case vwapData = <-vwapReq.Response:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for vwap data response")
	}

	reaction, err := shared.NewReactionAtVWAP(mkt.cfg.Market, vwapData, priceData)
	if err != nil {
		return fmt.Errorf("creating vwap reaction: %v", err)
	}

	m.cfg.SignalReactionAtVWAP(*reaction)
	select {
	case <-reaction.Status:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for reaction at vwap status")
	}

	mkt.ResetVWAPDataState()

	return nil
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

	err := m.evaluateReactionAtLevelSignal(mkt, candle.Timeframe)
	if err != nil {
		return fmt.Errorf("evaluating reaction at level signal: %v", err)
	}

	err = m.evaluateReactionAtVWAPSignal(mkt, candle.Timeframe)
	if err != nil {
		return fmt.Errorf("evaluating reaction at vwap signal: %v", err)
	}

	err = m.evaluateReactionAtImbalanceSignal(mkt, candle.Timeframe)
	if err != nil {
		return fmt.Errorf("evaluating reaction at imbalance signal: %v", err)
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

	level := shared.NewLevel(signal.Market, signal.Price, signal.Close)
	mkt.AddLevel(level)
	m.cfg.Logger.Info().Msgf("added new %s level @ %.2f for %s", level.Kind.String(), level.Price, level.Market)

	return nil
}

// handleImbalanceSignal processes the provided imbalance signal.
func (m *Manager) handleImbalanceSignal(signal shared.ImbalanceSignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	mkt, ok := m.markets[signal.Market]
	if !ok {
		return fmt.Errorf("no market found with name %s", signal.Market)
	}

	imb := &signal.Imbalance
	mkt.AddImbalance(imb)
	m.cfg.Logger.Info().Msgf("added new %s imbalance with gap ratio %.2f covering %.2f - %.2f for %s",
		imb.Sentiment.String(), imb.GapRatio, imb.High, imb.Low, imb.Market)

	return nil
}

// handleCandleMetadataRequest processes the provided candle metadata request.
func (m *Manager) handleCandleMetadataRequest(req *shared.CandleMetadataRequest) error {
	_, ok := m.markets[req.Market]
	if !ok {
		return fmt.Errorf("no market found with name: %s", req.Market)
	}

	// Request price data and generate price reactions from them.
	priceDataReq := shared.NewPriceDataRequest(req.Market, req.Timeframe, shared.PriceDataPayloadSize+1)
	m.cfg.RequestPriceData(*priceDataReq)
	var data []*shared.Candlestick
	select {
	case data = <-priceDataReq.Response:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out waiting for price data response")
	}

	// Generate metadata for all candles in the range being evaluated.
	metadataSet := make([]*shared.CandleMetadata, 0, shared.PriceDataPayloadSize)
	for idx := 1; idx < len(data)-1; idx++ {
		currentCandle := data[idx]
		previousCandle := data[idx-1]

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
		case signal := <-m.imbalanceSignals:
			m.workers[signal.Market] <- struct{}{}
			go func(signal shared.ImbalanceSignal) {
				err := m.handleImbalanceSignal(signal)
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
