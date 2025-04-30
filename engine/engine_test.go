package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupEngine() (*Engine, chan shared.AverageVolumeRequest, chan shared.CandleMetadataRequest, chan shared.MarketSkewRequest) {
	candleMetadataReqs := make(chan shared.CandleMetadataRequest, 5)
	requestCandleMetadata := func(req shared.CandleMetadataRequest) {
		candleMetadataReqs <- req
	}

	avgVolumeReqs := make(chan shared.AverageVolumeRequest, 5)
	requestAvgVolume := func(req shared.AverageVolumeRequest) {
		avgVolumeReqs <- req
	}

	entrySignals := make(chan shared.EntrySignal, 5)
	signalEntry := func(signal shared.EntrySignal) {
		entrySignals <- signal
	}

	exitSignals := make(chan shared.ExitSignal, 5)
	signalExit := func(signal shared.ExitSignal) {
		exitSignals <- signal
	}

	marketSkewReqs := make(chan shared.MarketSkewRequest, 5)
	requestMarketSkew := func(req shared.MarketSkewRequest) {
		marketSkewReqs <- req
	}

	cfg := &EngineConfig{
		RequestCandleMetadata: requestCandleMetadata,
		RequestAverageVolume:  requestAvgVolume,
		SendEntrySignal:       signalEntry,
		SendExitSignal:        signalExit,
		RequestMatketSkew:     requestMarketSkew,
		Logger:                log.Logger,
	}

	eng := NewEngine(cfg)

	return eng, avgVolumeReqs, candleMetadataReqs, marketSkewReqs
}
func TestEngine(t *testing.T) {
	eng, _, _, _ := setupEngine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure the engine can be run.

	done := make(chan struct{})
	go func() {
		eng.Run(ctx)
		close(done)
	}()

	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	eng, _, _, _ := setupEngine()

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
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		eng.SignalLevelReaction(&levelReaction)
	}

	assert.Equal(t, len(eng.levelReactionSignals), bufferSize)
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
	eng, _, _, _ := setupEngine()

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
	eng, _, _, _ := setupEngine()
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
	eng, _, _, _ := setupEngine()

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
	eng, _, _, _ := setupEngine()

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
	// Ensure two confluence points are awarded for a strong engulfing candle structure with high momemtum.
	err = eng.evaluateCandleMetadataStrength(highStrengthCandleMeta, reactionSentiment, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(3))
	assert.Equal(t, len(reasons), 2)

	keys := make([]shared.Reason, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}

	assert.Equal(t, keys, []shared.Reason{shared.StrongMove, shared.BullishEngulfing})
}

func TestEvaluatePriceReversalConfirmation(t *testing.T) {
	eng, _, _, _ := setupEngine()
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

	slice := reasonKeys(reasons)

	assert.Equal(t, slice[0], shared.ReversalAtSupport)

	// Ensure bearish price reactions can be confirmed.
	confluence = 0
	reasons = map[shared.Reason]struct{}{}
	sentiment = shared.Neutral
	err = eng.evaluatePriceReversalConfirmation(&resistanceLevelReaction, &confluence, &sentiment, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, sentiment, shared.Bearish)

	slice = reasonKeys(reasons)

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

func TestReasonKeys(t *testing.T) {
	reasons := map[shared.Reason]struct{}{}
	reasons[shared.BearishEngulfing] = struct{}{}
	reasons[shared.BreakAboveResistance] = struct{}{}

	// Ensure reasons are sliced as epxected from the provided map.
	slice := reasonKeys(reasons)
	assert.Equal(t, len(slice), 2)
	assert.Equal(t, slice, []shared.Reason{shared.BearishEngulfing, shared.BreakAboveResistance})
}

func TestFetchAverageVolume(t *testing.T) {
	eng, avgVolumeReqs, _, _ := setupEngine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case req := <-avgVolumeReqs:
				req.Response <- 10
			}
		}
	}()

	// Ensure average volume requests can be processed.
	market := "^GSPC"
	avgVol, err := eng.fetchAverageVolume(market)
	assert.NoError(t, err)
	assert.Equal(t, avgVol, float64(10))

	cancel()
	<-done
}

func TestFetchMarketSkew(t *testing.T) {
	eng, _, _, marketSkewReqs := setupEngine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case req := <-marketSkewReqs:
				req.Response <- shared.NeutralSkew
			}
		}
	}()

	// Ensure market skew requests can be processed.
	market := "^GSPC"
	avgVol, err := eng.fetchMarketSkew(market)
	assert.NoError(t, err)
	assert.Equal(t, avgVol, shared.NeutralSkew)

	cancel()
	<-done
}
