package engine

import (
	"context"
	"fmt"
	"math"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 16
	// minReversalConfluence is the minumum required confluence to confirm a reversal.
	minReversalConfluence = 7
	// minBreakConfluence is the minumum required confluence to confirm a break.
	minBreakConfluence = 7
	// minAverageVolumePercent is the minimum percentage above average volume to be considered
	// substantive.
	minAverageVolumePercent = float64(0.3)
	// stopLossBuffer is buffer for setting stoplosses in points.
	stopLossBuffer = float64(2)
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
	RequestMatketSkew func(request shared.MarketSkewRequest)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

type Engine struct {
	cfg                  *EngineConfig
	workers              chan struct{}
	levelReactionSignals chan *shared.LevelReaction
}

// NewEngine initializes a new market engine.
func NewEngine(cfg *EngineConfig) *Engine {
	return &Engine{
		cfg:                  cfg,
		workers:              make(chan struct{}, maxWorkers),
		levelReactionSignals: make(chan *shared.LevelReaction, bufferSize),
	}
}

// SignalLevelReaction relays the provided level reaction for processing.
func (e *Engine) SignalLevelReaction(reaction *shared.LevelReaction) {
	select {
	case e.levelReactionSignals <- reaction:
		// do nothing.
	default:
		e.cfg.Logger.Error().Msgf("price level reactions channel at capacity: %d/%d",
			len(e.levelReactionSignals), bufferSize)
	}
}

// evaluateHighVolumeSession awards confluence points if the provided level reaction occured during a high volume session.
func (e *Engine) evaluateHighVolumeSession(levelReaction *shared.LevelReaction, confluence *uint32, reasons map[shared.Reason]struct{}) error {
	// A reversal occuring during sessions known for high volume indicates strength.
	sessionName, err := shared.CurrentSession(levelReaction.CreatedOn)
	if err != nil {
		return fmt.Errorf("fetching current session: %v", err)
	}

	if sessionName == shared.London || sessionName == shared.NewYork {
		*confluence++
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
			*confluence += 2
			reasons[shared.StrongVolume] = struct{}{}
		case volumeDifference > 0:
			*confluence++
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
		*confluence++
		reasons[shared.StrongMove] = struct{}{}
	}

	// An engulfing reversal signifies directional strength.
	if candleMeta.Engulfing && (candleMeta.Momentum == shared.High || candleMeta.Momentum == shared.Medium) {
		*confluence += 2
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
func (e *Engine) evaluatePriceReversalConfirmation(levelReaction *shared.LevelReaction, confluence *uint32, sentiment *shared.Sentiment, reasons map[shared.Reason]struct{}) error {
	if levelReaction.Reaction != shared.Reversal {
		return fmt.Errorf("level reaction is not a reversal, got %s", levelReaction.Reaction.String())
	}

	// Confirmed price reactions at key levels indicate strength.
	switch levelReaction.Level.Kind {
	case shared.Resistance:
		*confluence++
		*sentiment = shared.Bearish
		reasons[shared.ReversalAtResistance] = struct{}{}
	case shared.Support:
		*confluence++
		*sentiment = shared.Bullish
		reasons[shared.ReversalAtSupport] = struct{}{}
	}

	return nil
}

// reasonKeys generates a reasons key slice from the provided map.
func reasonKeys(reasons map[shared.Reason]struct{}) []shared.Reason {
	data := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		data = append(data, k)
	}

	return data
}

// fetchAverageVolume fetches the average volume of the provided market.
func (e *Engine) fetchAverageVolume(market string) (float64, error) {
	req := shared.AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64, 1),
	}

	e.cfg.RequestAverageVolume(req)

	select {
	case averageVolume := <-req.Response:
		return averageVolume, nil
	case <-time.After(time.Second * 5):
		return 0, fmt.Errorf("timed out fetching average volume for %s", market)
	}
}

// fetchMarketSkew fetches the market skew for the provided market.
func (e *Engine) fetchMarketSkew(market string) (shared.MarketSkew, error) {
	req := shared.MarketSkewRequest{
		Market:   market,
		Response: make(chan shared.MarketSkew, 1),
	}

	e.cfg.RequestMatketSkew(req)

	select {
	case skew := <-req.Response:
		return skew, nil
	case <-time.After(time.Second * 5):
		return 0, fmt.Errorf("timed out fetching market skew for %s", market)
	}
}

// evaluatePriceReversal determines whether an actionable price reversal has occured.
func (e *Engine) evaluatePriceReversal(market string, meta []*shared.CandleMetadata, levelReaction *shared.LevelReaction) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluence uint32
	var reactionSentiment shared.Sentiment
	reasonsKV := make(map[shared.Reason]struct{})

	// Confirmed price reactions at key levels indicate strength.
	err := e.evaluatePriceReversalConfirmation(levelReaction, &confluence, &reactionSentiment, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating price reversal confirmation: %v", err)
	}

	// A reversal occuring during sessions known for high volume indicates strength.
	err = e.evaluateHighVolumeSession(levelReaction, &confluence, reasonsKV)
	if err != nil {
		return false, 0, nil, fmt.Errorf("evaluating high volume session: %v", err)
	}

	averageVolume, err := e.fetchAverageVolume(market)
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
		if averageVolume > 0 {
			volumeDiff := candleMeta.Volume - averageVolume

			err = e.evaluateVolumeStrength(averageVolume, volumeDiff, &confluence, reasonsKV)
			if err != nil {
				return false, 0, nil, fmt.Errorf("evaluating volume strength: %v", err)
			}
		}
	}

	signal := confluence >= minReversalConfluence

	reasons := reasonKeys(reasonsKV)

	return signal, confluence, reasons, nil
}

// evaluateBreak determines whether an actionable break has occured.
func (e *Engine) evaluateBreak(market string, meta []*shared.CandleMetadata, sentiment shared.Sentiment) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluences uint32
	reasons := make(map[shared.Reason]struct{})

	// A break must oppose the provided level.
	lastMeta := meta[len(meta)-1]
	if lastMeta.Sentiment != sentiment {
		confluences++
		switch sentiment {
		case shared.Bullish:
			reasons[shared.BreakAboveResistance] = struct{}{}
		case shared.Bearish:
			reasons[shared.BreakBelowSupport] = struct{}{}
		}
	}

	// A break occuring during sessions known for high volume indicates strength.
	sessionName, err := shared.CurrentSession(lastMeta.Date)
	if err != nil {
		return false, 0, nil, fmt.Errorf("fetching current session: %v", err)
	}

	if sessionName == shared.London || sessionName == shared.NewYork {
		confluences++
		reasons[shared.HighVolumeSession] = struct{}{}
	}

	req := shared.AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64),
	}

	e.cfg.RequestAverageVolume(req)
	averageVolume := <-req.Response

	for idx := 0; idx < len(meta); idx++ {
		// Only evaluate candle meta that support the sentiment of the reaction.
		if meta[idx].Sentiment != sentiment {
			continue
		}

		// A break must show strength (candle structure and momentum) in order to be actionable.
		if (meta[idx].Kind == shared.Marubozu || meta[idx].Kind == shared.Pinbar) &&
			(meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences++
			reasons[shared.StrongMove] = struct{}{}
		}

		// An engulfing break signifies directional strength.
		if meta[idx].Engulfing && (meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences += 2
			switch meta[idx].Sentiment {
			case shared.Bullish:
				reasons[shared.BullishEngulfing] = struct{}{}
			case shared.Bearish:
				reasons[shared.BearishEngulfing] = struct{}{}
			}
		}

		// A break with above average volume signifies strength.
		if averageVolume > 0 {
			volumeDiff := meta[idx].Volume - averageVolume

			switch {
			case volumeDiff/averageVolume >= minAverageVolumePercent:
				// A break substantially above average volume is a great indicator of strength.
				confluences += 2
				reasons[shared.StrongVolume] = struct{}{}
			case volumeDiff > 0:
				confluences++
				reasons[shared.StrongVolume] = struct{}{}
			}
		}
	}

	signal := confluences >= minBreakConfluence

	reasonSet := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		reasonSet = append(reasonSet, k)
	}

	return signal, confluences, reasonSet, nil
}

// estimateStopLoss calculates the stoploss and the point range from entry for a position using
// the provided high, low and position direction.
func (e *Engine) estimateStopLoss(high float64, low float64, entry float64, direction shared.Direction) (float64, float64, error) {
	// some sanity checks.
	if low > high {
		return 0, 0, fmt.Errorf("low (%.2f) cannot be greater than high (%.2f)", low, high)
	}
	if entry > high {
		return 0, 0, fmt.Errorf("entry (%.2f) cannot be greater than high (%.2f)", entry, high)
	}
	if entry < low {
		return 0, 0, fmt.Errorf("entry (%.2f) cannot be less than low (%.2f)", entry, low)
	}

	var stopLoss, pointsRange float64

	switch direction {
	case shared.Long:
		stopLoss = low - stopLossBuffer
	case shared.Short:
		stopLoss = high + stopLossBuffer
	}

	pointsRange = math.Abs(entry - stopLoss)

	if stopLoss < 0 {
		return 0, 0, fmt.Errorf("stop loss cannot be zero")
	}

	return stopLoss, pointsRange, nil
}

// evaluatePriceReversalStrength determines whether a price reversal at a level has enough confluences to
// be classified as strong. An associated entry or exit signal is generated and relayed for it based on
// the state of the associated market.
func (e *Engine) evaluatePriceReversalStrength(reaction *shared.LevelReaction, meta []*shared.CandleMetadata, high float64, low float64) error {
	signal, confluences, reasons, err := e.evaluatePriceReversal(reaction.Market, meta, reaction)
	if err != nil {
		return fmt.Errorf("evaluating reversal level reaction: %v", err)
	}

	if signal {
		skew, err := e.fetchMarketSkew(reaction.Market)
		if err != nil {
			return fmt.Errorf("fetching market skew : %v", err)
		}

		switch {
		case (skew == shared.NeutralSkew || skew == shared.LongSkewed) && reaction.Level.Kind == shared.Support:
			// Signal a long position on a confirmed support reversal if the market is
			// neutral skewed or already long skewed.
			direction := shared.Long
			stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, direction)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)

		case skew == shared.LongSkewed && reaction.Level.Kind == shared.Resistance:
			// A confirmed resistance reversal for a long skewed market acts as an exit condition.
			direction := shared.Long
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)

		case (skew == shared.NeutralSkew || skew == shared.ShortSkewed) && reaction.Level.Kind == shared.Resistance:
			// Signal a short position on a confirmed resistance reversal if the market is
			// neutral skewed or already short skewed.
			direction := shared.Short
			stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, direction)
			if err != nil {
				return fmt.Errorf("estimating stop loss: %v", err)
			}

			signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
			e.cfg.SendEntrySignal(signal)

		case skew == shared.LongSkewed && reaction.Level.Kind == shared.Resistance:
			// A confirmed support reversal for a short skewed market acts as an exit condition.
			direction := shared.Short
			signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, direction,
				reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
			e.cfg.SendExitSignal(signal)
		}
	}

	return nil
}

// evaluateLevelBreakStrength determines whether a level break has enough confluences to be
// classified as strong. An associated entry or exit signal is generated and relayed for it based on
// the state of the associated market.
func (e *Engine) evaluateLevelBreakStrength(reaction *shared.LevelReaction, meta []*shared.CandleMetadata, high float64, low float64) error {
	switch reaction.Level.Kind {
	case shared.Support:
		signal, confluences, reasons, err := e.evaluateBreak(reaction.Market, meta, shared.Bearish)
		if err != nil {
			return fmt.Errorf("evaluating break level reaction: %v", err)
		}

		if signal {
			req := shared.MarketStatusRequest{
				Market:   reaction.Market,
				Response: make(chan shared.MarketStatus),
			}

			e.cfg.RequestMatketStatus(req)
			status := <-req.Response

			switch status {
			case shared.NeutralInclination:
				// Signal a short position on a high confluence support break if the market is
				// neutral directionally.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Short)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.ShortInclined:
				// Add to the short market inclination by signalling a short position on a
				// high confluence support break.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Short)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.LongInclined:
				// A high confluence support break for a long inclined market indicates a
				// good exit condition.
				signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, shared.Long,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
				e.cfg.SendExitSignal(signal)
			}
		}
	case shared.Resistance:
		signal, confluences, reasons, err := e.evaluateBreak(reaction.Market, meta, shared.Bullish)
		if err != nil {
			return fmt.Errorf("evaluating break level reaction: %v", err)
		}

		if signal {
			req := shared.MarketStatusRequest{
				Market:   reaction.Market,
				Response: make(chan shared.MarketStatus),
			}

			e.cfg.RequestMatketStatus(req)
			status := <-req.Response

			switch status {
			case shared.NeutralInclination:
				// Signal a long position on a high confluence resistance break if the market is
				// neutral directionally.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Long)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Long,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.LongInclined:
				// Add to the long market inclination by signalling a long position on a
				// high confluence resistance break.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Long)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Long,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.ShortInclined:
				// A high confluence resistance break for a short inclined market indicates a
				// good exit condition.
				signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
				e.cfg.SendExitSignal(signal)
			}
		}
	default:
		// do nothing.
	}

	return nil
}

// handleLevelReaction processes the provided level reaction.
func (e *Engine) handleLevelReaction(reaction *shared.LevelReaction) error {
	// Fetch the current candle's metadata.
	req := shared.CandleMetadataRequest{
		Market:   reaction.Market,
		Response: make(chan []*shared.CandleMetadata),
	}

	e.cfg.RequestCandleMetadata(req)

	meta := <-req.Response

	high, low := shared.CandleMetaRangeHighAndLow(meta)

	switch reaction.Reaction {
	case shared.Reversal:
		err := e.evaluateLevelReversalStrength(reaction, meta, high, low)
		if err != nil {
			return fmt.Errorf("evaluating level reversal strength: %v", err)
		}
	case shared.Break:
		err := e.evaluateLevelBreakStrength(reaction, meta, high, low)
		if err != nil {
			return fmt.Errorf("evaluating level break strength: %v", err)
		}
	case shared.Chop:
		// Do nothing.
		e.cfg.Logger.Info().Msgf("chop level reaction encountered for market %s", reaction.Market)
	}

	reaction.ApplyReaction()

	return nil
}

// Run manages the lifecycle processes of the market engine.
func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-e.levelReactionSignals:
			// use workers to process level reactions concurrently.
			e.workers <- struct{}{}
			go func(signal *shared.LevelReaction) {
				err := e.handleLevelReaction(signal)
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
