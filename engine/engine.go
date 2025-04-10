package engine

import (
	"context"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 16
	// minReversalConfluence is the minumum required confluence to confirm a reversal.
	minReversalConfluence = 4
	// minBreakConfluence is the minumum required confluence to confirm a break.
	minBreakConfluence = 5
	// minAverageVolumePercent is the minimum percentage above average volume to be considered
	// substantive.
	minAverageVolumePercent = float64(0.2)
)

type EngineConfig struct {
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// RequestCandleMetadata relays the provided candle metadata request for processing.
	RequestCandleMetadata func(req shared.CandleMetadataRequest)
	// RequestAverageVolume relays the provided average volume request for processing.
	RequestAverageVolume func(request *shared.AverageVolumeRequest)
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
func (e *Engine) evaluateReversal(market string, meta *shared.CandleMetadata, sentiment shared.Sentiment) bool {
	var confluences uint32

	// A reversal must confirm the provided level.
	if meta.Sentiment == sentiment {
		confluences++
	}

	// A reversal must show stregnth in order to be actionable.
	if meta.Kind == shared.Marubozu || meta.Kind == shared.Pinbar {
		confluences++
	}

	// A high momentum reversal signifies strength.
	if meta.Momentum == shared.High || meta.Momentum == shared.Medium {
		confluences++
	}

	// An engulfing reversal signifies directional strength.
	if meta.Engulfing && (meta.Momentum == shared.High || meta.Momentum == shared.Medium) {
		confluences += 2
	}

	resp := make(chan float64)
	req := &shared.AverageVolumeRequest{
		Market:   market,
		Response: &resp,
	}

	e.cfg.RequestAverageVolume(req)
	averageVolume := <-resp

	// A reversal with above average volume signifies strength.
	volumeDiff := meta.Volume - averageVolume
	if volumeDiff > 0 {
		confluences++
	}

	// A reversal substantially above average volume is a great indicator of strength.
	if volumeDiff/averageVolume >= minAverageVolumePercent {
		confluences++
	}

	return confluences >= minReversalConfluence
}

// evaluateBreak determines whether an actionable break has occured.
func (e *Engine) evaluateBreak(market string, meta *shared.CandleMetadata, sentiment shared.Sentiment) bool {
	var confluences uint32

	// A break must oppose the provided level.
	if meta.Sentiment == sentiment {
		confluences++
	}

	// A break must show stregnth in order to be actionable.
	if meta.Kind == shared.Marubozu {
		confluences++
	}

	// A high momentum break signifies strength.
	if meta.Momentum == shared.High || meta.Momentum == shared.Medium {
		confluences++
	}

	// An engulfing reversal signifies directional strength.
	if meta.Engulfing && (meta.Momentum == shared.High || meta.Momentum == shared.Medium) {
		confluences += 2
	}

	resp := make(chan float64)
	req := &shared.AverageVolumeRequest{
		Market:   market,
		Response: &resp,
	}

	e.cfg.RequestAverageVolume(req)
	averageVolume := <-resp

	// A level break with above average volume signifies strength.
	volumeDiff := meta.Volume - averageVolume
	if volumeDiff > 0 {
		confluences++
	}

	// A level break substantially above average volume is a great indicator of strength.
	if volumeDiff/averageVolume >= minAverageVolumePercent {
		confluences++
	}

	return confluences >= minBreakConfluence
}

// handleLevelReaction processes the provided level reaction.
func (e *Engine) handleLevelReaction(reaction *shared.LevelReaction) {
	// Fetch the current candle's metadata.
	resp := make(chan shared.CandleMetadata)
	req := shared.CandleMetadataRequest{
		Market:   reaction.Market,
		Response: &resp,
	}

	e.cfg.RequestCandleMetadata(req)

	meta := <-resp

	switch reaction.Reaction {
	case shared.Reversal:
		switch reaction.Level.Kind {
		case shared.Support:
			if e.evaluateReversal(reaction.Market, &meta, shared.Bullish) {
				// todo: signal a bullish reversal setup.
			}
		case shared.Resistance:
			if e.evaluateReversal(reaction.Market, &meta, shared.Bearish) {
				// todo: signal a bearish reversal setup.
			}
		default:
			// do nothing.
		}
	case shared.Break:
		switch reaction.Level.Kind {
		case shared.Support:
			if e.evaluateBreak(reaction.Market, &meta, shared.Bearish) {
				// todo: signal a bearish break setup.
			}

		case shared.Resistance:
			if e.evaluateBreak(reaction.Market, &meta, shared.Bullish) {
				// todo: signal a bullish break setup.
			}
		}
	case shared.Chop:
		// Do nothing.
	}
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
