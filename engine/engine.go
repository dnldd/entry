package engine

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
	maxWorkers = 16
	// minReversalConfluence is the minumum required confluence to confirm a reversal.
	minReversalConfluence = 8
	// minBreakConfluence is the minumum required confluence to confirm a break.
	minBreakConfluence = 8
	// minAverageVolumePercent is the minimum percentage above average volume to be considered
	// substantive.
	minAverageVolumePercent = float64(0.3)
)

type EngineConfig struct {
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// RequestCandleMetadata relays the provided candle metadata request for processing.
	RequestCandleMetadata func(req shared.CandleMetadataRequest)
	// RequestAverageVolume relays the provided average volume request for processing.
	RequestAverageVolume func(request *shared.AverageVolumeRequest)
	// SendEntrySignal relays the provided entry signal for processing.
	SendEntrySignal func(signal shared.EntrySignal)
	// SendExitSignal relays the provided exit signal for processing.
	SendExitSignal func(signal shared.ExitSignal)
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
func (e *Engine) SignalLevelReaction(signal *shared.LevelReaction) {
	select {
	case e.levelReactionSignals <- signal:
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
	reasons := []shared.Reason{}

	// A reversal must confirm the provided level.
	lastMeta := meta[len(meta)-1]
	if lastMeta.Sentiment == sentiment {
		confluences++
		switch sentiment {
		case shared.Bullish:
			reasons = append(reasons, shared.ReversalAtSupport)
		case shared.Bearish:
			reasons = append(reasons, shared.ReversalAtResistance)
		}
	}

	// A reversal occuring during sessions known for high volume indicates strength.
	sessionName, err := shared.CurrentSession(lastMeta.Date)
	if err != nil {
		return false, 0, nil, fmt.Errorf("fetching current session: %v", err)
	}

	if sessionName == shared.London || sessionName == shared.NewYork {
		confluences++
		reasons = append(reasons, shared.HighVolumeSession)
	}

	req := &shared.AverageVolumeRequest{
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
			reasons = append(reasons, shared.StrongMove)
		}

		// An engulfing reversal signifies directional strength.
		if meta[idx].Engulfing && (meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences += 2
			switch meta[idx].Sentiment {
			case shared.Bullish:
				reasons = append(reasons, shared.BullishEngulfing)
			case shared.Bearish:
				reasons = append(reasons, shared.BearishEngulfing)
			}
		}

		// A reversal with above average volume signifies strength.
		if averageVolume > 0 {
			volumeDiff := meta[idx].Volume - averageVolume

			switch {
			case volumeDiff/averageVolume >= minAverageVolumePercent:
				// A reversal substantially above average volume is a great indicator of strength.
				confluences += 2
				reasons = append(reasons, shared.StrongVolume)
			case volumeDiff > 0:
				confluences++
				reasons = append(reasons, shared.StrongVolume)
			}
		}
	}

	signal := confluences >= minReversalConfluence

	return signal, confluences, reasons, nil
}

// evaluateBreak determines whether an actionable break has occured.
func (e *Engine) evaluateBreak(market string, meta []*shared.CandleMetadata, sentiment shared.Sentiment) (bool, uint32, []shared.Reason, error) {
	if len(meta) == 0 {
		return false, 0, nil, fmt.Errorf("candle metadata is empty")
	}

	var confluences uint32
	reasons := []shared.Reason{}

	// A break must oppose the provided level.
	lastMeta := meta[len(meta)-1]
	if lastMeta.Sentiment != sentiment {
		confluences++
		switch sentiment {
		case shared.Bullish:
			reasons = append(reasons, shared.BreakAboveResistance)
		case shared.Bearish:
			reasons = append(reasons, shared.BreakBelowSupport)
		}
	}

	// A break occuring during sessions known for high volume indicates strength.
	sessionName, err := shared.CurrentSession(lastMeta.Date)
	if err != nil {
		return false, 0, nil, fmt.Errorf("fetching current session: %v", err)
	}

	if sessionName == shared.London || sessionName == shared.NewYork {
		confluences++
		reasons = append(reasons, shared.HighVolumeSession)
	}

	req := &shared.AverageVolumeRequest{
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
			reasons = append(reasons, shared.StrongMove)
		}

		// An engulfing break signifies directional strength.
		if meta[idx].Engulfing && (meta[idx].Momentum == shared.High || meta[idx].Momentum == shared.Medium) {
			confluences += 2
			switch meta[idx].Sentiment {
			case shared.Bullish:
				reasons = append(reasons, shared.BullishEngulfing)
			case shared.Bearish:
				reasons = append(reasons, shared.BearishEngulfing)
			}
		}

		// A break with above average volume signifies strength.
		if averageVolume > 0 {
			volumeDiff := meta[idx].Volume - averageVolume

			switch {
			case volumeDiff/averageVolume >= minAverageVolumePercent:
				// A break substantially above average volume is a great indicator of strength.
				confluences += 2
				reasons = append(reasons, shared.StrongVolume)
			case volumeDiff > 0:
				confluences++
				reasons = append(reasons, shared.StrongVolume)
			}
		}
	}

	signal := confluences >= minBreakConfluence

	return signal, confluences, reasons, nil
}

// handleLevelReaction processes the provided level reaction.
func (e *Engine) handleLevelReaction(reaction *shared.LevelReaction) {
	// Fetch the current candle's metadata.
	req := shared.CandleMetadataRequest{
		Market:   reaction.Market,
		Response: make(chan []*shared.CandleMetadata),
	}

	e.cfg.RequestCandleMetadata(req)

	meta := <-req.Response

	switch reaction.Reaction {
	case shared.Reversal:
		switch reaction.Level.Kind {
		case shared.Support:
			signal, confluences, reasons := e.evaluateReversal(reaction.Market, meta, shared.Bullish)
			if signal {

				// todo: signal a bullish reversal setup.
			}
		case shared.Resistance:
			signal, confluences, reasons := e.evaluateReversal(reaction.Market, meta, shared.Bearish)
			if signal {
				// todo: signal a bearish reversal setup.
			}
		default:
			// do nothing.
		}
	case shared.Break:
		switch reaction.Level.Kind {
		case shared.Support:
			signal, confluences, reasons := e.evaluateBreak(reaction.Market, meta, shared.Bearish)
			if signal {
				// todo: signal a bearish break setup.
			}

		case shared.Resistance:
			signal, confluences, reasons := e.evaluateBreak(reaction.Market, meta, shared.Bullish)
			if signal {
				// todo: signal a bullish break setup.
			}
		}
	case shared.Chop:
		// Do nothing.
	}

	reaction.ApplyReaction()
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
				e.handleLevelReaction(signal)
				<-e.workers
			}(signal)
		default:
			// fallthrough
		}
	}
}
