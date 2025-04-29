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
	// RequestMarketStatus relays the provided market status request for processing.
	RequestMatketStatus func(request shared.MarketStatusRequest)
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

// evaluateReversal determines whether an actionable reversal has occured.
func (e *Engine) evaluateReversal(market string, meta []*shared.CandleMetadata, sentiment shared.Sentiment) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluences uint32
	reasons := make(map[shared.Reason]struct{})

	// A reversal must confirm the provided level.
	lastMeta := meta[len(meta)-1]
	if lastMeta.Sentiment == sentiment {
		confluences++
		switch sentiment {
		case shared.Bullish:
			reasons[shared.ReversalAtSupport] = struct{}{}
		case shared.Bearish:
			reasons[shared.ReversalAtResistance] = struct{}{}
		}
	}

	// A reversal occuring during sessions known for high volume indicates strength.
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

		// A reversal must show strength (candle structure and momentum) in order to be actionable.
		if (meta[idx].Kind == shared.Marubozu || meta[idx].Kind == shared.Pinbar) &&
			(meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences++
			reasons[shared.StrongMove] = struct{}{}
		}

		// An engulfing reversal signifies directional strength.
		if meta[idx].Engulfing && (meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences += 2
			switch meta[idx].Sentiment {
			case shared.Bullish:
				reasons[shared.BullishEngulfing] = struct{}{}
			case shared.Bearish:
				reasons[shared.BearishEngulfing] = struct{}{}
			}
		}

		// A reversal with above average volume signifies strength.
		if averageVolume > 0 {
			volumeDiff := meta[idx].Volume - averageVolume

			switch {
			case volumeDiff/averageVolume >= minAverageVolumePercent:
				// A reversal substantially above average volume is a great indicator of strength.
				confluences += 2
				reasons[shared.StrongVolume] = struct{}{}
			case volumeDiff > 0:
				confluences++
				reasons[shared.StrongVolume] = struct{}{}
			}
		}
	}

	signal := confluences >= minReversalConfluence

	reasonSet := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		reasonSet = append(reasonSet, k)
	}

	return signal, confluences, reasonSet, nil
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

// calculateStopLoss calculates the stoploss and the point range from entry for a position using
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

// evaluateLevelReversalStrength determines whether a price reversal at a level has enough confluences to
// be classified as strong. An associated entry or exit signal is generated and relayed for it based on
// the state of the associated market.
func (e *Engine) evaluateLevelReversalStrength(reaction *shared.LevelReaction, meta []*shared.CandleMetadata, high float64, low float64) error {
	switch reaction.Level.Kind {
	case shared.Support:
		signal, confluences, reasons, err := e.evaluateReversal(reaction.Market, meta, shared.Bullish)
		if err != nil {
			return fmt.Errorf("evaluating reversal level reaction: %v", err)
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
				// Signal a long position on a high confluence support reversal if the market is
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
				// high confluence support reversal.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Long)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Long,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.ShortInclined:
				// A high confluence support reversal for a short inclined market indicates a
				// good exit condition.
				signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
				e.cfg.SendExitSignal(signal)
			}
		}
	case shared.Resistance:
		signal, confluences, reasons, err := e.evaluateReversal(reaction.Market, meta, shared.Bearish)
		if err != nil {
			return fmt.Errorf("evaluating reversal level reaction: %v", err)
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
				// Signal a short position on a high confluence resistance reversal if the
				// market is neutral directionally.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Short)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.ShortInclined:
				// Add to the short market inclination by signalling a short position on a
				// high confluence resistance reversal.
				stopLoss, pointsRange, err := e.estimateStopLoss(high, low, reaction.CurrentPrice, shared.Short)
				if err != nil {
					return err
				}

				signal := shared.NewEntrySignal(reaction.Market, reaction.Timeframe, shared.Short,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn, stopLoss, pointsRange)
				e.cfg.SendEntrySignal(signal)

			case shared.LongInclined:
				// A high confluence resistance reversal for a long inclined market indicates a
				// good exit condition.
				signal := shared.NewExitSignal(reaction.Market, reaction.Timeframe, shared.Long,
					reaction.CurrentPrice, reasons, confluences, reaction.CreatedOn)
				e.cfg.SendExitSignal(signal)
			}
		}
	default:
		// do nothing.
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
