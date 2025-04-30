package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupEngine(avgVolume *float64, candleMeta []*shared.CandleMetadata, marketSkew *shared.MarketSkew) (*Engine, chan shared.EntrySignal, chan shared.ExitSignal) {
	requestCandleMetadata := func(req shared.CandleMetadataRequest) {
		req.Response <- candleMeta
	}
	requestAvgVolume := func(req shared.AverageVolumeRequest) {
		req.Response <- *avgVolume
	}

	entrySignals := make(chan shared.EntrySignal, 5)
	signalEntry := func(signal shared.EntrySignal) {
		entrySignals <- signal
	}

	exitSignals := make(chan shared.ExitSignal, 5)
	signalExit := func(signal shared.ExitSignal) {
		exitSignals <- signal
	}

	requestMarketSkew := func(req shared.MarketSkewRequest) {
		req.Response <- *marketSkew
	}

	cfg := &EngineConfig{
		RequestCandleMetadata: requestCandleMetadata,
		RequestAverageVolume:  requestAvgVolume,
		SendEntrySignal:       signalEntry,
		SendExitSignal:        signalExit,
		RequestMarketSkew:     requestMarketSkew,
		Logger:                log.Logger,
	}

	eng := NewEngine(cfg)

	return eng, entrySignals, exitSignals
}

func TestEngine(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure the engine can be run.

	done := make(chan struct{})
	go func() {
		eng.Run(ctx)
		close(done)
	}()

	asiaSessionTime, _ := generateSessionTimes(t)

	// Ensure the engine can handle a level reaction signal.
	market := "^GSPC"
	levelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asiaSessionTime,
		Status:        make(chan shared.StatusCode, 1),
	}

	eng.SignalLevelReaction(levelReaction)
	<-levelReaction.Status

	time.Sleep(time.Millisecond * 400)

	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	market := "^GSPC"
	candle := shared.Candlestick{
		Open:      12,
		High:      15,
		Low:       9,
		Close:     11,
		Volume:    2,
		Date:      now,
		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	levelPrice := float64(8)
	level := shared.NewLevel(market, levelPrice, &candle)
	levelReaction := shared.LevelReaction{
		Market:        market,
		Timeframe:     shared.FiveMinute,
		Level:         level,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		CurrentPrice:  10,
		Reaction:      shared.Reversal,
		CreatedOn:     now,
		Status:        make(chan shared.StatusCode, 1),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		eng.SignalLevelReaction(&levelReaction)
	}

	assert.Equal(t, len(eng.levelReactionSignals), bufferSize)
}

func TestHandleLevelReaction(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asiaSessionTime, _ := generateSessionTimes(t)

	market := "^GSPC"
	priceReversalReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asiaSessionTime,
		Status:        make(chan shared.StatusCode, 1),
	}

	// Ensure the engine can handle a price reversal level reaction signal.
	eng.handleLevelReaction(priceReversalReaction)
	<-priceReversalReaction.Status

	breakLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Resistance,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Break,
		CreatedOn:     asiaSessionTime,
		Status:        make(chan shared.StatusCode, 1),
	}

	// Ensure the engine can handle a break level reaction signal.
	eng.handleLevelReaction(breakLevelReaction)
	<-breakLevelReaction.Status

	chopLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Resistance,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Below, shared.Above, shared.Below},
		Reaction:      shared.Chop,
		CreatedOn:     asiaSessionTime,
		Status:        make(chan shared.StatusCode, 1),
	}

	// Ensure the engine can handle a break chop level reaction signal.
	eng.handleLevelReaction(chopLevelReaction)
	<-chopLevelReaction.Status
}

func generateSessionTimes(t *testing.T) (time.Time, time.Time) {
	now, loc, err := shared.NewYorkTime()
	assert.NoError(t, err)

	asiaSessionStr := "18:30"
	asiaSession, err := time.Parse(shared.SessionTimeLayout, asiaSessionStr)
	assert.NoError(t, err)
	asiaSessionTime := time.Date(now.Year(), now.Month(), now.Day(), asiaSession.Hour(), asiaSession.Minute(), 0, 0, loc)

	londonSessionStr := "03:30"
	londonSession, err := time.Parse(shared.SessionTimeLayout, londonSessionStr)
	assert.NoError(t, err)
	londonSessionTime := time.Date(now.Year(), now.Month(), now.Day(), londonSession.Hour(), londonSession.Minute(), 0, 0, loc)

	return asiaSessionTime, londonSessionTime
}

func TestEstimateStopLoss(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	tests := []struct {
		name        string
		high        float64
		low         float64
		entry       float64
		direction   shared.Direction
		wantErr     bool
		stoploss    float64
		pointsRange float64
	}{
		{
			"negative stop loss",
			float64(2),
			float64(1),
			float64(1.2),
			shared.Long,
			true,
			0,
			0,
		},
		{
			"low is greater than high",
			float64(2),
			float64(4),
			float64(5),
			shared.Long,
			true,
			0,
			0,
		},
		{
			"entry is greater than high",
			float64(5),
			float64(4),
			float64(7),
			shared.Long,
			true,
			0,
			0,
		},
		{
			"entry is less than low",
			float64(5),
			float64(4),
			float64(3),
			shared.Long,
			true,
			0,
			0,
		},
		{
			"valid stop loss (long)",
			float64(8),
			float64(4),
			float64(5),
			shared.Long,
			false,
			2.0,
			3.0,
		},
		{
			"valid stop loss (short)",
			float64(10),
			float64(7),
			float64(9),
			shared.Short,
			false,
			12.0,
			3.0,
		},
	}

	for _, test := range tests {
		sl, pr, err := eng.estimateStopLoss(test.high, test.low, test.entry, test.direction)
		if test.wantErr && err == nil {
			t.Errorf("%s: expected an error, got none", test.name)
		}

		if !test.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		if !test.wantErr {
			if sl != test.stoploss {
				t.Errorf("%s: expected stop loss %.2f, got %.2f", test.name, test.stoploss, sl)
			}

			if pr != test.pointsRange {
				t.Errorf("%s: expected points range %.2f, got %.2f", test.name, test.pointsRange, pr)
			}
		}
	}
}

func TestEvaluateHighVolumeSession(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asianSessionTime, londonSessionTime := generateSessionTimes(t)
	market := "^GSPC"
	levelReaction := shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asianSessionTime,
	}

	// Ensure confluence points are not awarded for asian session.
	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}
	err := eng.evaluateHighVolumeSession(&levelReaction, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(0))
	assert.Equal(t, len(reasons), 0)

	// Ensure confluence points are awarded for high volume sessions (london & new york)
	levelReaction.CreatedOn = londonSessionTime

	err = eng.evaluateHighVolumeSession(&levelReaction, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, len(reasons), 1)

	keys := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}

	assert.Equal(t, keys[0], shared.HighVolumeSession)
}

func TestEvaluateVolumeStrength(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	averageVolume := float64(10)
	volumeDifference := float64(-2)
	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}

	// Ensure no confluence points are awarded for a volume difference below the average volume.
	err := eng.evaluateVolumeStrength(averageVolume, volumeDifference, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(0))
	assert.Equal(t, len(reasons), 0)

	// Ensure a confluence point is awarded for a volume difference above the average volume but
	// below the volume percent threshold.
	volumeDifference = float64(2.5)
	err = eng.evaluateVolumeStrength(averageVolume, volumeDifference, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, len(reasons), 1)

	keys := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}

	assert.Equal(t, keys[0].String(), shared.StrongVolume.String())

	// Ensure two confluence points are awarded for a volume difference above the average and above
	// the volume percent threshold.
	volumeDifference = float64(4)
	confluence = uint32(0)
	err = eng.evaluateVolumeStrength(averageVolume, volumeDifference, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(2))
	assert.Equal(t, len(reasons), 1)

	keys = make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}

	assert.Equal(t, keys[0], shared.StrongVolume)
}

func TestEvaluateCandleVolumeStrength(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}
	reactionSentiment := shared.Bullish
	mediumStrengthCandleMeta := shared.CandleMetadata{
		Kind:      shared.Pinbar,
		Sentiment: shared.Bearish,
		Momentum:  shared.Medium,
		Volume:    float64(4),
		Engulfing: false,
		High:      float64(8),
		Low:       float64(2),
	}

	// Ensure only candle metadata supporting the reaction sentiment are evaluated.
	err := eng.evaluateCandleMetadataStrength(mediumStrengthCandleMeta, reactionSentiment, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(0))
	assert.Equal(t, len(reasons), 0)

	// Ensure a confluence point is awarded for a strong candle structure with medium momemtum.
	reactionSentiment = shared.Bearish
	err = eng.evaluateCandleMetadataStrength(mediumStrengthCandleMeta, reactionSentiment, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, len(reasons), 1)

	highStrengthCandleMeta := shared.CandleMetadata{
		Kind:      shared.Marubozu,
		Sentiment: shared.Bullish,
		Momentum:  shared.High,
		Volume:    float64(10),
		Engulfing: true,
		High:      float64(8),
		Low:       float64(2),
	}

	confluence = uint32(0)
	reactionSentiment = shared.Bullish
	// Ensure a confluence point is awarded for a strong engulfing candle structure with high momemtum.
	err = eng.evaluateCandleMetadataStrength(highStrengthCandleMeta, reactionSentiment, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(2))
	assert.Equal(t, len(reasons), 2)

	keys := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}

	assert.In(t, shared.StrongMove, keys)
	assert.In(t, shared.BullishEngulfing, keys)
}

func TestEvaluatePriceReversalConfirmation(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asianSessionTime, _ := generateSessionTimes(t)

	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}
	sentiment := shared.Neutral
	market := "^GSPC"
	supportLevelReaction := shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asianSessionTime,
	}

	resistanceLevelReaction := shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Resistance,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Below, shared.Below, shared.Below, shared.Below},
		Reaction:      shared.Reversal,
		CreatedOn:     asianSessionTime,
	}

	// Ensure bullish price reactions can be confirmed.
	err := eng.evaluatePriceReversalConfirmation(&supportLevelReaction, &confluence, &sentiment, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, sentiment, shared.Bullish)

	slice := extractReasons(reasons)

	assert.Equal(t, slice[0], shared.ReversalAtSupport)

	// Ensure bearish price reactions can be confirmed.
	confluence = 0
	reasons = map[shared.Reason]struct{}{}
	sentiment = shared.Neutral
	err = eng.evaluatePriceReversalConfirmation(&resistanceLevelReaction, &confluence, &sentiment, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, sentiment, shared.Bearish)

	slice = extractReasons(reasons)

	assert.Equal(t, slice[0], shared.ReversalAtResistance)

	// Ensure the reversal confirmation errors if the level reaction is not a reversal.
	invalidReversalLevelReaction := shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Break,
		CreatedOn:     asianSessionTime,
	}

	err = eng.evaluatePriceReversalConfirmation(&invalidReversalLevelReaction, &confluence, &sentiment, reasons)
	assert.Error(t, err)

}

func TestExtractReasons(t *testing.T) {
	reasons := map[shared.Reason]struct{}{}
	reasons[shared.BearishEngulfing] = struct{}{}
	reasons[shared.BreakAboveResistance] = struct{}{}

	// Ensure reasons are sliced as epxected from the provided map.
	slice := extractReasons(reasons)
	assert.Equal(t, len(slice), 2)
	assert.In(t, shared.BearishEngulfing, slice)
	assert.In(t, shared.BreakAboveResistance, slice)
}

func TestFetchAverageVolume(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	// Ensure average volume requests can be processed.
	market := "^GSPC"
	avgVol, err := eng.fetchAverageVolume(market)
	assert.NoError(t, err)
	assert.Equal(t, avgVol, float64(10))
}

func TestFetchCandleMetadata(t *testing.T) {
	avgVolume := float64(10)
	asiaSessionTime, _ := generateSessionTimes(t)
	candleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bearish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      5,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      6,
			Low:       2,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(5),
			Engulfing: false,
			High:      6,
			Low:       2,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      14,
			Low:       6,
			Date:      asiaSessionTime,
		},
	}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	// Ensure average volume requests can be processed.
	market := "^GSPC"
	meta, err := eng.fetchCandleMetadata(market)
	assert.NoError(t, err)
	assert.Equal(t, len(meta), 4)
}

func TestFetchMarketSkew(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	// Ensure market skew requests can be processed.
	market := "^GSPC"
	avgVol, err := eng.fetchMarketSkew(market)
	assert.NoError(t, err)
	assert.Equal(t, avgVol, shared.NeutralSkew)
}

func TestEvaluatePriceReversal(t *testing.T) {
	avgVolume := float64(4)
	asiaSessionTime, _ := generateSessionTimes(t)
	candleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bearish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      5,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      6,
			Low:       2,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(5),
			Engulfing: false,
			High:      6,
			Low:       2,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      14,
			Low:       6,
			Date:      asiaSessionTime,
		},
	}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)
	market := "^GSPC"
	levelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asiaSessionTime,
	}

	// Ensure price reversal is not evaluated if the meta is an empty slice.
	signal, _, _, err := eng.evaluatePriceReversal(levelReaction, []*shared.CandleMetadata{})
	assert.Error(t, err)

	// Ensure price reversal is evualuated as expected with valid input.
	signal, confluence, reasons, err := eng.evaluatePriceReversal(levelReaction, candleMeta)
	assert.NoError(t, err)
	assert.In(t, shared.ReversalAtSupport, reasons)
	assert.In(t, shared.StrongMove, reasons)
	assert.In(t, shared.StrongVolume, reasons)
	assert.Equal(t, confluence, uint32(7))
	assert.Equal(t, signal, true)
}

func TestEvaluateLevelBreak(t *testing.T) {
	avgVolume := float64(4)
	asiaSessionTime, _ := generateSessionTimes(t)
	candleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      8,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(6),
			Engulfing: false,
			High:      12,
			Low:       8,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(6),
			Engulfing: false,
			High:      15,
			Low:       8,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      18,
			Low:       15,
			Date:      asiaSessionTime,
		},
	}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)
	market := "^GSPC"
	levelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(5),
			Kind:   shared.Resistance,
		},
		CurrentPrice:  float64(18),
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Break,
		CreatedOn:     asiaSessionTime,
	}

	// Ensure price break is not evaluated if the meta is an empty slice.
	signal, _, _, err := eng.evaluateLevelBreak(levelReaction, []*shared.CandleMetadata{})
	assert.Error(t, err)

	// Ensure price reversal is evualuated as expected with valid input.
	signal, confluence, reasons, err := eng.evaluateLevelBreak(levelReaction, candleMeta)
	assert.NoError(t, err)
	assert.In(t, shared.BreakAboveResistance, reasons)
	assert.In(t, shared.StrongMove, reasons)
	assert.In(t, shared.StrongVolume, reasons)
	assert.Equal(t, confluence, uint32(10))
	assert.Equal(t, signal, true)
}

func TestEvaluatePriceReversalStrength(t *testing.T) {
	avgVolume := float64(4)
	asiaSessionTime, _ := generateSessionTimes(t)
	supportCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bearish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      5,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      6,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(5),
			Engulfing: false,
			High:      9,
			Low:       6,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      14,
			Low:       9,
			Date:      asiaSessionTime,
		},
	}

	resistanceCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      11,
			Low:       9,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      9,
			Low:       7,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(5),
			Engulfing: false,
			High:      7,
			Low:       5,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      6,
			Low:       1,
			Date:      asiaSessionTime,
		},
	}

	candleMeta := supportCandleMeta
	longSkew := shared.LongSkewed
	shortSkew := shared.ShortSkewed
	marketSkew := longSkew
	eng, entrySignals, exitSignals := setupEngine(&avgVolume, candleMeta, &marketSkew)
	market := "^GSPC"
	supportLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(3),
			Kind:   shared.Support,
		},
		CurrentPrice:  float64(14),
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Reversal,
		CreatedOn:     asiaSessionTime,
	}

	resistanceLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(10),
			Kind:   shared.Resistance,
		},
		CurrentPrice:  float64(1),
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Below, shared.Below, shared.Below, shared.Below},
		Reaction:      shared.Reversal,
		CreatedOn:     asiaSessionTime,
	}

	// Ensure a support price reversal triggers a long entry signal for a market long or neutral skewed.
	high := float64(14)
	low := float64(3)
	err := eng.evaluatePriceReversalStrength(supportLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	entrySignal := <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Long)

	// Ensure a support price reversal triggers a short exit signal for a market short skewed.
	marketSkew = shortSkew
	err = eng.evaluatePriceReversalStrength(supportLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	exitSignal := <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Short)

	// Ensure a resistance price reversal triggers a long exit signal for a market long skewed.
	high = 11
	low = 1
	marketSkew = longSkew
	candleMeta = resistanceCandleMeta
	err = eng.evaluatePriceReversalStrength(resistanceLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	exitSignal = <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Long)

	// Ensure a resistance price reversal triggers a short entry signal for a market short or neutral skewed.
	marketSkew = shortSkew
	candleMeta = resistanceCandleMeta
	err = eng.evaluatePriceReversalStrength(resistanceLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	entrySignal = <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Short)
}

func TestEvaluateLevelBreakStrength(t *testing.T) {
	avgVolume := float64(4)
	asiaSessionTime, _ := generateSessionTimes(t)
	resistanceBreakCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      8,
			Low:       4,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(6),
			Engulfing: false,
			High:      12,
			Low:       8,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(6),
			Engulfing: false,
			High:      15,
			Low:       8,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      18,
			Low:       15,
			Date:      asiaSessionTime,
		},
	}

	supportBreakCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bearish,
			Momentum:  shared.Low,
			Volume:    float64(1),
			Engulfing: false,
			High:      11,
			Low:       9,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      9,
			Low:       7,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(5),
			Engulfing: false,
			High:      7,
			Low:       5,
			Date:      asiaSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.High,
			Volume:    float64(8),
			Engulfing: false,
			High:      6,
			Low:       1,
			Date:      asiaSessionTime,
		},
	}

	candleMeta := supportBreakCandleMeta
	longSkew := shared.LongSkewed
	shortSkew := shared.ShortSkewed
	marketSkew := shortSkew
	eng, entrySignals, exitSignals := setupEngine(&avgVolume, candleMeta, &marketSkew)
	market := "^GSPC"
	supportLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(10),
			Kind:   shared.Support,
		},
		CurrentPrice:  float64(1),
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Below, shared.Below, shared.Below, shared.Below},
		Reaction:      shared.Break,
		CreatedOn:     asiaSessionTime,
	}

	resistanceLevelReaction := &shared.LevelReaction{
		Market: market,
		Level: &shared.Level{
			Market: market,
			Price:  float64(5),
			Kind:   shared.Resistance,
		},
		CurrentPrice:  float64(18),
		Timeframe:     shared.FiveMinute,
		PriceMovement: []shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above},
		Reaction:      shared.Break,
		CreatedOn:     asiaSessionTime,
	}

	// Ensure a support price break triggers a short entry signal for a market short or neutral skewed.
	high := float64(16)
	low := float64(1)
	err := eng.evaluateLevelBreakStrength(supportLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	entrySignal := <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Short)

	// Ensure a support price break triggers a short exit signal for a market long skewed.
	marketSkew = longSkew
	err = eng.evaluateLevelBreakStrength(supportLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	exitSignal := <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Long)

	// Ensure a resistance level break triggers a long entry signal for a market long skewed.
	high = float64(18)
	low = float64(4)
	candleMeta = resistanceBreakCandleMeta
	err = eng.evaluateLevelBreakStrength(resistanceLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	entrySignal = <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Long)

	// Ensure a resistance level break triggers a short exit signal for a market short skewed.
	marketSkew = shortSkew
	err = eng.evaluateLevelBreakStrength(resistanceLevelReaction, candleMeta, high, low)
	assert.NoError(t, err)
	exitSignal = <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Short)
}
