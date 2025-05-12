package engine

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 16
	// minLevelReversalConfluence is the minumum required confluence to confirm a level reversal.
	minLevelReversalConfluence = 6
	// minBreakConfluence is the minumum required confluence to confirm a level break.
	minLevelBreakConfluence = 6
	// minVWAPReversalConfluence is the minumum required confluence to confirm a vwap reversal.
	minVWAPReversalConfluence = 6
	// minBreakConfluence is the minumum required confluence to confirm a vwap break.
	minVWAPBreakConfluence = 6
	// minAverageVolumePercent is the minimum percentage above average volume to be considered
	// substantive.
	minAverageVolumePercent = float64(0.3)
	// stopLossBuffer is buffer for setting stoplosses in points.
	stopLossPointsBuffer = float64(1)
)

type EngineConfig struct {
	// RequestCandleMetadata relays the provided candle metadata request for processing.
	RequestCandleMetadata func(req shared.CandleMetadataRequest)
	// RequestAverageVolume relays the provided average volume request for processing.
	RequestAverageVolume func(request shared.AverageVolumeRequest)
	// SendEntrySignal relays the provided entry signal for processing.
	SendEntrySignal func(signal shared.EntrySignal)
	// SendExitSignal relays the provided exit signal for processing.
	SendExitSignal func(signal shared.ExitSignal)
	// RequestMarketSkew relays the provided market skew request for processing.
	RequestMarketSkew func(request shared.MarketSkewRequest)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

type Engine struct {
	cfg                    *EngineConfig
	workers                chan struct{}
	reactionAtLevelSignals chan shared.ReactionAtLevel
	reactionAtVWAPSignals  chan shared.ReactionAtVWAP
}

// NewEngine initializes a new market engine.
func NewEngine(cfg *EngineConfig) *Engine {
	return &Engine{
		cfg:                    cfg,
		workers:                make(chan struct{}, maxWorkers),
		reactionAtLevelSignals: make(chan shared.ReactionAtLevel, bufferSize),
		reactionAtVWAPSignals:  make(chan shared.ReactionAtVWAP, bufferSize),
	}
}

// SignalReactionAtLevel relays the provided reaction at level for processing.
func (e *Engine) SignalReactionAtLevel(reaction shared.ReactionAtLevel) {
	select {
	case e.reactionAtLevelSignals <- reaction:
		// do nothing.
	default:
		e.cfg.Logger.Error().Msgf("reaction at level signals channel at capacity: %d/%d",
			len(e.reactionAtLevelSignals), bufferSize)
	}
}

// SignalReactionAtVWAP relays the provided reaction at VWAP for processing.
func (e *Engine) SignalReactionAtVWAP(reaction shared.ReactionAtVWAP) {
	select {
	case e.reactionAtVWAPSignals <- reaction:
		// do nothing.
	default:
		e.cfg.Logger.Error().Msgf("reaction a vwap signals channel at capacity: %d/%d",
			len(e.reactionAtVWAPSignals), bufferSize)
	}
}

// evaluateHighVolumeSession awards confluence points if the provided time occured during a high volume session.
func (e *Engine) evaluateHighVolumeSession(reaction *shared.ReactionAtFocus, confluence *uint32, reasons map[shared.Reason]struct{}) error {
	// Any notable price action move occuring during the high volume window indicates strength.
	highVolumeWindow, err := shared.InHighVolumeWindow(reaction.CreatedOn)
	if err != nil {
		return fmt.Errorf("checking high volume window status: %v", err)
	}

	if highVolumeWindow {
		(*confluence)++
		reasons[shared.HighVolumeSession] = struct{}{}
	}

	return nil
}

// evaluateVolumeStrength awards confluence points if the provided volume difference is greater than the provided average volume.
func (e *Engine) evaluateVolumeStrength(averageVolume float64, volumeDifference float64, confluence *uint32, reasons map[shared.Reason]struct{}) error {
	// A break with above average volume signifies strength.
	if averageVolume > 0 {
		switch {
		case volumeDifference/averageVolume >= minAverageVolumePercent:
			// A break substantially above average volume is a great indicator of strength.
			(*confluence) += 2
			reasons[shared.StrongVolume] = struct{}{}
		case volumeDifference > 0:
			(*confluence)++
			reasons[shared.StrongVolume] = struct{}{}
		}
	}

	return nil
}

// evaluateCandleMetadataStrength awards confluence points based on the provided candle structure and momentum.
func (e *Engine) evaluateCandleMetadataStrength(candleMeta shared.CandleMetadata, reactionSentiment shared.Sentiment, confluence *uint32, reasons map[shared.Reason]struct{}) error {
	// Only evaluate candle metadata that supports the sentiment of the reaction.
	if candleMeta.Sentiment != reactionSentiment {
		// do nothing.
		return nil
	}

	// A reversal must show strength (candle structure and momentum) in order to be actionable.
	if (candleMeta.Kind == shared.Marubozu || candleMeta.Kind == shared.Pinbar) &&
		(candleMeta.Momentum == shared.High || candleMeta.Momentum == shared.Medium) {
		(*confluence)++
		reasons[shared.StrongMove] = struct{}{}
	}

	// An engulfing reversal signifies directional strength.
	if candleMeta.Engulfing && (candleMeta.Momentum == shared.High || candleMeta.Momentum == shared.Medium) {
		(*confluence)++
		switch candleMeta.Sentiment {
		case shared.Bullish:
			reasons[shared.BullishEngulfing] = struct{}{}
		case shared.Bearish:
			reasons[shared.BearishEngulfing] = struct{}{}
		}
	}

	return nil
}

// evaluatePriceReversalConfirmation awards confluence points based on confirmation of the level reaction being a reversal.
func (e *Engine) evaluatePriceReversalConfirmation(reaction *shared.ReactionAtFocus, confluence *uint32, reactionSentiment *shared.Sentiment, reasons map[shared.Reason]struct{}) error {
	if reaction.Reaction != shared.Reversal {
		return fmt.Errorf("level reaction is not a reversal, got %s", reaction.Reaction.String())
	}

	// Confirmed price reversals at key levels indicate strength.
	switch reaction.LevelKind {
	case shared.Resistance:
		*confluence++
		*reactionSentiment = shared.Bearish
		reasons[shared.ReversalAtResistance] = struct{}{}
	case shared.Support:
		*confluence++
		*reactionSentiment = shared.Bullish
		reasons[shared.ReversalAtSupport] = struct{}{}
	default:
		return fmt.Errorf("unknown level kind provided: %s", reaction.LevelKind.String())
	}

	return nil
}

// extractReasons generates a reasons key slice from the provided map.
func extractReasons(reasons map[shared.Reason]struct{}) []shared.Reason {
	data := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		data = append(data, k)
	}

	return data
}

// fetchAverageVolume fetches the average volume of the provided market.
func (e *Engine) fetchAverageVolume(market string) (float64, error) {
	req := shared.NewAverageVolumeRequest(market)
	e.cfg.RequestAverageVolume(*req)

	select {
	case averageVolume := <-req.Response:
		return averageVolume, nil
	case <-time.After(time.Second * 5):
		return 0, fmt.Errorf("timed out fetching average volume for %s", market)
	}
}

// fetchMarketSkew fetches the market skew for the provided market.
func (e *Engine) fetchMarketSkew(market string) (shared.MarketSkew, error) {
	req := shared.NewMarketSkewRequest(market)
	e.cfg.RequestMarketSkew(*req)

	select {
	case skew := <-req.Response:
		return skew, nil
	case <-time.After(time.Second * 5):
		return 0, fmt.Errorf("timed out fetching market skew for %s", market)
	}
}

// fetchCandleMetadata fetches the candle metadata for the provided market.
func (e *Engine) fetchCandleMetadata(market string) ([]*shared.CandleMetadata, error) {
	req := shared.NewCandleMetadataRequest(market)
	e.cfg.RequestCandleMetadata(*req)

	select {
	case meta := <-req.Response:
		return meta, nil
	case <-time.After(time.Second * 5):
		return nil, fmt.Errorf("timed out fetching candle metadata for %s", market)
	}
}

// evaluatePriceReversal determines whether an actionable price reversal has occured.
func (e *Engine) evaluatePriceReversal(reaction *shared.ReactionAtFocus, meta []*shared.CandleMetadata, minConfluenceThreshold uint32) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluence uint32
	var reactionSentiment shared.Sentiment
	reasonsKV := make(map[shared.Reason]struct{})

	// Confirmed price reactions at key focus indicate strength.
	err := e.evaluatePriceReversalConfirmation(reaction, &confluence, &reactionSentiment, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating price reversal confirmation: %v", err)
	}

	// A reversal occuring during sessions known for high volume indicates strength.
	err = e.evaluateHighVolumeSession(reaction, &confluence, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating high volume session: %v", err)
	}

	averageVolume, err := e.fetchAverageVolume(reaction.Market)
	if err != nil {
		return false, 0, nil, fmt.Errorf("fetching average volume: %v", err)
	}

	for idx := range meta {
		candleMeta := meta[idx]

		err = e.evaluateCandleMetadataStrength(*candleMeta, reactionSentiment, &confluence, reasonsKV)
		if err != nil {
			return false, 0, nil, fmt.Errorf("evaluating candle metadata strength: %v", err)
		}

		// A reversal with above average volume signifies strength.
		volumeDiff := candleMeta.Volume - averageVolume
		err = e.evaluateVolumeStrength(averageVolume, volumeDiff, &confluence, reasonsKV)
		if err != nil {
			return false, 0, nil, fmt.Errorf("evaluating volume strength: %v", err)
		}
	}

	signal := confluence >= minConfluenceThreshold

	reasons := extractReasons(reasonsKV)

	return signal, confluence, reasons, nil
}

// evaluateLevelBreakConfirmation awards confluence points based on confirmation of the level reaction being a break.
func (e *Engine) evaluateBreakConfirmation(reaction *shared.ReactionAtFocus, confluence *uint32, reactionSentiment *shared.Sentiment, reasons map[shared.Reason]struct{}) error {
	if reaction.Reaction != shared.Break {
		return fmt.Errorf("level reaction is not a break, got %s", reaction.Reaction.String())
	}

	// Confirmed breaks at key levels indicate strength.
	switch reaction.LevelKind {
	case shared.Resistance:
		*confluence++
		*reactionSentiment = shared.Bullish
		reasons[shared.BreakAboveResistance] = struct{}{}
	case shared.Support:
		*confluence++
		*reactionSentiment = shared.Bearish
		reasons[shared.BreakBelowSupport] = struct{}{}
	}

	return nil
}

// evaluateLevelBreak determines whether an actionable level break has occured.
func (e *Engine) evaluateLevelBreak(reaction *shared.ReactionAtFocus, meta []*shared.CandleMetadata, minConfluenceThreshold uint32) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluence uint32
	var reactionSentiment shared.Sentiment
	reasonsKV := make(map[shared.Reason]struct{})

	// Confirmed breaks at key focus indicate strength.
	err := e.evaluateBreakConfirmation(reaction, &confluence, &reactionSentiment, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating level break confirmation: %v", err)
	}

	// A reversal occuring during sessions known for high volume indicates strength.
	err = e.evaluateHighVolumeSession(reaction, &confluence, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating high volume session: %v", err)
	}

	averageVolume, err := e.fetchAverageVolume(reaction.Market)
	if err != nil {
		return false, 0, nil, fmt.Errorf("fetching average volume: %v", err)
	}

	for idx := range meta {
		candleMeta := meta[idx]

		err = e.evaluateCandleMetadataStrength(*candleMeta, reactionSentiment, &confluence, reasonsKV)
		if err != nil {
			return false, 0, nil, fmt.Errorf("evaluating candle metadata strength: %v", err)
		}

		// A break with above average volume signifies strength.
		volumeDiff := meta[idx].Volume - averageVolume
		err = e.evaluateVolumeStrength(averageVolume, volumeDiff, &confluence, reasonsKV)
		if err != nil {
			return false, 0, nil, fmt.Errorf("evaluating volume strength: %v", err)
		}
	}

	signal := confluence >= minConfluenceThreshold

	reasons := extractReasons(reasonsKV)

	return signal, confluence, reasons, nil
}

// estimateStopLoss calculates the stoploss and the point range from entry for a position using
// the provided candle metadata.
func (e *Engine) estimateStopLoss(reaction *shared.ReactionAtFocus, meta []*shared.CandleMetadata) (float64, float64, error) {
	if len(meta) == 0 {
		return 0, 0, fmt.Errorf("no candle metadata provided")
	}

	// Derive the directional sentiment from the level reaction.
	var sentiment shared.Sentiment
	switch reaction.LevelKind {
	case shared.Support:
		switch reaction.Reaction {
		case shared.Break:
			sentiment = shared.Bearish
		case shared.Reversal:
			sentiment = shared.Bullish
		case shared.Chop:
			return 0, 0, fmt.Errorf("no stop loss set for chop level reaction")
		}
	case shared.Resistance:
		switch reaction.Reaction {
		case shared.Break:
			sentiment = shared.Bullish
		case shared.Reversal:
			sentiment = shared.Bearish
		case shared.Chop:
			return 0, 0, fmt.Errorf("no stop loss set for chop level reaction")
		}
	}

	var stopLoss float64

	signalCandle := shared.FetchSignalCandle(meta, sentiment)
	if signalCandle == nil {
		// Fallback on the high and low of the candle metadata range for stop loss placement.
		high, low := shared.CandleMetaRangeHighAndLow(meta)
		switch sentiment {
		case shared.Bullish:
			stopLoss = low - stopLossPointsBuffer
		case shared.Bearish:
			stopLoss = high + stopLossPointsBuffer
		}

	} else {
		// Use the signal candle as the focal point for the stop loss placement.
		switch sentiment {
		case shared.Bullish:
			stopLoss = signalCandle.Low - stopLossPointsBuffer
		case shared.Bearish:
			stopLoss = signalCandle.High + stopLossPointsBuffer
		}
	}

	pointsRange := math.Abs(reaction.CurrentPrice - stopLoss)

	if stopLoss <= 0 {
		return 0, 0, fmt.Errorf("stop loss cannot be less than or equal to zero")
	}

	return stopLoss, pointsRange, nil
}

// evaluatePriceReversalStrength determines whether a price reversal at a level has enough confluences to
// be classified as strong. An associated entry or exit signal is generated and relayed for it based on
// the skew of the associated market.
func (e *Engine) evaluatePriceReversalStrength(reaction *shared.ReactionAtFocus, meta []*shared.CandleMetadata, minConfluenceThreshold uint32) error {
	signal, confluence, reasons, err := e.evaluatePriceReversal(reaction, meta, minConfluenceThreshold)
	if err != nil {
		return fmt.Errorf("evaluating price reversal reaction: %v", err)
	}

	e.cfg.Logger.Info().Msgf("price reversal confluence – (%d), signal status – %v", confluence, signal)

	if signal {
		skew, err := e.fetchMarketSkew(reaction.Market)
		if err != nil {
			return fmt.Errorf("fetching market skew: %v", err)
		}

		switch {
		case (skew == shared.NeutralSkew || skew == shared.LongSkewed) && reaction.LevelKind == shared.Support:
			// Signal a long position on a confirmed support level reversal if the market is
			// neutral skewed or already long skewed.
			direction := shared.Long
			stopLoss, pointsRange, err := e.estimateStopLoss(reaction, meta)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)
			select {
			case <-signal.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out waiting for entry signal status")
			}

		case skew == shared.LongSkewed && reaction.LevelKind == shared.Resistance:
			// A confirmed resistance level reversal for a long skewed market acts as an exit condition.
			direction := shared.Long
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)
			select {
			case <-signal.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out waiting for entry signal status")
			}

		case (skew == shared.NeutralSkew || skew == shared.ShortSkewed) && reaction.LevelKind == shared.Resistance:
			// Signal a short position on a confirmed resistance reversal if the market is
			// neutral skewed or already short skewed.
			direction := shared.Short
			stopLoss, pointsRange, err := e.estimateStopLoss(reaction, meta)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)
			select {
			case <-signal.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out waiting for entry signal status")
			}

		case skew == shared.ShortSkewed && reaction.LevelKind == shared.Support:
			// A confirmed support reversal for a short skewed market acts as an exit condition.
			direction := shared.Short
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)
			select {
			case <-signal.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out waiting for entry signal status")
			}
		}
	}

	return nil
}

// evaluateBreakStrength determines whether a break has enough confluences to be
// classified as strong. An associated entry or exit signal is generated and relayed for it based on
// the skew of the associated market.
func (e *Engine) evaluateBreakStrength(reaction *shared.ReactionAtFocus, meta []*shared.CandleMetadata, minConfluenceThreshold uint32) error {
	signal, confluence, reasons, err := e.evaluateLevelBreak(reaction, meta, minConfluenceThreshold)
	if err != nil {
		return fmt.Errorf("evaluating break reaction: %v", err)
	}

	e.cfg.Logger.Info().Msgf("break confluence – (%d), signal status – %v", confluence, signal)

	if signal {
		skew, err := e.fetchMarketSkew(reaction.Market)
		if err != nil {
			return fmt.Errorf("fetching market skew: %v", err)
		}

		switch {
		case (skew == shared.NeutralSkew || skew == shared.LongSkewed) && reaction.LevelKind == shared.Resistance:
			// Signal a long position on a confirmed resistance level break if the market is
			// neutral skewed or already long skewed.
			direction := shared.Long
			stopLoss, pointsRange, err := e.estimateStopLoss(reaction, meta)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)
		case skew == shared.LongSkewed && reaction.LevelKind == shared.Support:
			// A confirmed support break for a long skewed market acts as an exit condition.
			direction := shared.Long
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)
		case (skew == shared.NeutralSkew || skew == shared.ShortSkewed) && reaction.LevelKind == shared.Support:
			// Signal a short position on a confirmed support break if the market is
			// neutral skewed or already short skewed.
			direction := shared.Short
			stopLoss, pointsRange, err := e.estimateStopLoss(reaction, meta)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)

		case skew == shared.ShortSkewed && reaction.LevelKind == shared.Resistance:
			// A confirmed resistance break for a short skewed market acts as an exit condition.
			direction := shared.Short
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluence, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)
		}
	}

	return nil
}

// handleReactionAtLevel processes the provided reaction at level signal.
func (e *Engine) handleReactionAtLevel(reaction *shared.ReactionAtLevel) error {
	defer func() {
		reaction.Status <- shared.Processed
	}()

	e.cfg.Logger.Info().Msgf("%s level reaction detected @ %.2f",
		reaction.Level.Kind.String(), reaction.Level.Price)

	meta, err := e.fetchCandleMetadata(reaction.Market)
	if err != nil {
		return fmt.Errorf("fetching candle metadata: %v", err)
	}

	switch reaction.Reaction {
	case shared.Reversal:
		err := e.evaluatePriceReversalStrength(&reaction.ReactionAtFocus, meta, minLevelReversalConfluence)
		if err != nil {
			return fmt.Errorf("evaluating price reversal at vwap strength: %v", err)
		}
	case shared.Break:
		err := e.evaluateBreakStrength(&reaction.ReactionAtFocus, meta, minLevelBreakConfluence)
		if err != nil {
			return fmt.Errorf("evaluating level break strength: %v", err)
		}
	case shared.Chop:
		// Do nothing.
		e.cfg.Logger.Info().Msgf("chop level reaction encountered for market %s", reaction.Market)
	}

	reaction.ApplyPriceReaction()

	return nil
}

// handleReactionAtVWAP processes the provided reaction at vwap signal.
func (e *Engine) handleReactionAtVWAP(reaction *shared.ReactionAtVWAP) error {
	defer func() {
		reaction.Status <- shared.Processed
	}()

	e.cfg.Logger.Info().Msgf("vwap reaction detected @ %.2f", reaction.VWAPData[0].Value)

	meta, err := e.fetchCandleMetadata(reaction.Market)
	if err != nil {
		return fmt.Errorf("fetching candle metadata: %v", err)
	}

	switch reaction.Reaction {
	case shared.Reversal:
		err := e.evaluatePriceReversalStrength(&reaction.ReactionAtFocus, meta, minVWAPReversalConfluence)
		if err != nil {
			return fmt.Errorf("evaluating price reversal at vwap strength: %v", err)
		}
	case shared.Break:
		err := e.evaluateBreakStrength(&reaction.ReactionAtFocus, meta, minVWAPBreakConfluence)
		if err != nil {
			return fmt.Errorf("evaluating vwap break strength: %v", err)
		}
	case shared.Chop:
		// Do nothing.
		e.cfg.Logger.Info().Msgf("chop level reaction encountered for market %s", reaction.Market)
	}

	return nil
}

// Run manages the lifecycle processes of the market engine.
func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-e.reactionAtLevelSignals:
			// use workers to process reactions at levels concurrently.
			e.workers <- struct{}{}
			go func(signal shared.ReactionAtLevel) {
				err := e.handleReactionAtLevel(&signal)
				if err != nil {
					e.cfg.Logger.Error().Err(err).Send()
				}
				<-e.workers
			}(signal)
		case signal := <-e.reactionAtVWAPSignals:
			// use workers to process reactions at vwap concurrently.
			e.workers <- struct{}{}
			go func(signal shared.ReactionAtVWAP) {
				err := e.handleReactionAtVWAP(&signal)
				if err != nil {
					e.cfg.Logger.Error().Err(err).Send()
				}
				<-e.workers
			}(signal)
		default:
			// fallthrough
		}
	}
}
