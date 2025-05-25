package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
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
		signal.Status <- shared.Processed
		entrySignals <- signal
	}

	exitSignals := make(chan shared.ExitSignal, 5)
	signalExit := func(signal shared.ExitSignal) {
		signal.Status <- shared.Processed
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
	levelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
	}

	eng.SignalReactionAtLevel(levelReaction)
	<-levelReaction.Status

	// Ensure the engine candle handle a vwap reaction.
	vwapReaction := shared.ReactionAtVWAP{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		VWAPData: []*shared.VWAP{
			{
				Value: 2,
				Date:  time.Time{},
			},
			{
				Value: 2.1,
				Date:  time.Time{},
			},
			{
				Value: 2.2,
				Date:  time.Time{},
			},
			{
				Value: 2.3,
				Date:  time.Time{},
			},
		},
	}

	eng.SignalReactionAtVWAP(vwapReaction)
	<-vwapReaction.Status

	// Ensure the engine candle handle an imbalance reaction.
	imbalanceReaction := shared.ReactionAtImbalance{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Imbalance: &shared.Imbalance{
			Market:      market,
			High:        float64(10),
			Low:         float64(4),
			Midpoint:    float64(7),
			Timeframe:   shared.FiveMinute,
			Sentiment:   shared.Bullish,
			GapRatio:    float64(0.8),
			Purged:      *atomic.NewBool(false),
			Invalidated: *atomic.NewBool(false),
			Date:        time.Time{},
		},
	}

	eng.SignalReactionAtImbalance(imbalanceReaction)
	<-imbalanceReaction.Status

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
	close := float64(11)

	levelPrice := float64(8)
	level := shared.NewLevel(market, levelPrice, close)
	levelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     level.Kind,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			CurrentPrice:  10,
			Reaction:      shared.Reversal,
			CreatedOn:     now,
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: level,
	}

	vwapReaction := shared.ReactionAtVWAP{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			Status:        make(chan shared.StatusCode, 1),
		},
		VWAPData: []*shared.VWAP{
			{
				Value: 2,
				Date:  time.Time{},
			},
			{
				Value: 2.1,
				Date:  time.Time{},
			},
			{
				Value: 2.2,
				Date:  time.Time{},
			},
			{
				Value: 2.3,
				Date:  time.Time{},
			},
		},
	}

	// Ensure the engine candle handle an imbalance reaction.
	imbalanceReaction := shared.ReactionAtImbalance{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			Status:        make(chan shared.StatusCode, 1),
		},
		Imbalance: &shared.Imbalance{
			Market:      market,
			High:        float64(10),
			Low:         float64(4),
			Midpoint:    float64(7),
			Timeframe:   shared.FiveMinute,
			Sentiment:   shared.Bullish,
			GapRatio:    float64(0.8),
			Purged:      *atomic.NewBool(false),
			Invalidated: *atomic.NewBool(false),
			Date:        time.Time{},
		},
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		eng.SignalReactionAtLevel(levelReaction)
		eng.SignalReactionAtVWAP(vwapReaction)
		eng.SignalReactionAtImbalance(imbalanceReaction)
	}

	assert.Equal(t, len(eng.reactionAtLevelSignals), bufferSize)
	assert.Equal(t, len(eng.reactionAtVWAPSignals), bufferSize)
	assert.Equal(t, len(eng.reactionAtImbalanceSignals), bufferSize)
}

func TestHandleLevelReaction(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asiaSessionTime, _ := generateSessionTimes(t)

	market := "^GSPC"
	priceReversalReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
	}

	// Ensure the engine can handle a price reversal level reaction signal.
	eng.handleReactionAtLevel(priceReversalReaction)
	<-priceReversalReaction.Status

	breakLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Resistance,
		},
	}

	// Ensure the engine can handle a break level reaction signal.
	eng.handleReactionAtLevel(breakLevelReaction)
	<-breakLevelReaction.Status

	chopLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Below, shared.Above, shared.Below},
			Reaction:      shared.Chop,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Resistance,
		},
	}

	// Ensure the engine can handle a break chop level reaction signal.
	eng.handleReactionAtLevel(chopLevelReaction)
	<-chopLevelReaction.Status
}

func TestHandleVWAPReaction(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asiaSessionTime, _ := generateSessionTimes(t)

	market := "^GSPC"
	reversalVWAPReaction := shared.ReactionAtVWAP{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		VWAPData: []*shared.VWAP{
			{
				Value: 2,
				Date:  time.Time{},
			},
			{
				Value: 2.1,
				Date:  time.Time{},
			},
			{
				Value: 2.2,
				Date:  time.Time{},
			},
			{
				Value: 2.3,
				Date:  time.Time{},
			},
		},
	}

	// Ensure the engine can handle a reversal vwap reaction signal.
	eng.handleReactionAtVWAP(&reversalVWAPReaction)
	<-reversalVWAPReaction.Status

	breakVWAPReaction := shared.ReactionAtVWAP{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		VWAPData: []*shared.VWAP{
			{
				Value: 10,
				Date:  time.Time{},
			},
			{
				Value: 10.1,
				Date:  time.Time{},
			},
			{
				Value: 10.2,
				Date:  time.Time{},
			},
			{
				Value: 10.3,
				Date:  time.Time{},
			},
		},
	}

	// Ensure the engine can handle a vwap break reaction signal.
	eng.handleReactionAtVWAP(&breakVWAPReaction)
	<-breakVWAPReaction.Status

	chopVWAPReaction := shared.ReactionAtVWAP{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Below, shared.Above, shared.Below},
			Reaction:      shared.Chop,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		VWAPData: []*shared.VWAP{
			{
				Value: 10,
				Date:  time.Time{},
			},
			{
				Value: 10.1,
				Date:  time.Time{},
			},
			{
				Value: 10.2,
				Date:  time.Time{},
			},
			{
				Value: 10.3,
				Date:  time.Time{},
			},
		},
	}

	// Ensure the engine can handle a vwap chop reaction signal.
	eng.handleReactionAtVWAP(&chopVWAPReaction)
	<-chopVWAPReaction.Status
}

func TestHandleImbalanceReaction(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asiaSessionTime, _ := generateSessionTimes(t)

	market := "^GSPC"
	reversalImbalanceReaction := shared.ReactionAtImbalance{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Imbalance: &shared.Imbalance{
			Market:      market,
			High:        float64(10),
			Low:         float64(4),
			Midpoint:    float64(7),
			Timeframe:   shared.FiveMinute,
			Sentiment:   shared.Bullish,
			GapRatio:    float64(0.8),
			Purged:      *atomic.NewBool(false),
			Invalidated: *atomic.NewBool(false),
			Date:        asiaSessionTime,
		},
	}

	// Ensure the engine can handle an imbalance reversal reaction signal.
	eng.handleReactionAtImbalance(&reversalImbalanceReaction)
	<-reversalImbalanceReaction.Status

	breakImbalanceReaction := shared.ReactionAtImbalance{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Imbalance: &shared.Imbalance{
			Market:      market,
			High:        float64(10),
			Low:         float64(4),
			Midpoint:    float64(7),
			Timeframe:   shared.FiveMinute,
			Sentiment:   shared.Bullish,
			GapRatio:    float64(0.8),
			Purged:      *atomic.NewBool(false),
			Invalidated: *atomic.NewBool(false),
			Date:        asiaSessionTime,
		},
	}

	// Ensure the engine can handle an imbalance break reaction signal.
	eng.handleReactionAtImbalance(&breakImbalanceReaction)
	<-breakImbalanceReaction.Status

	chopImbalanceReaction := shared.ReactionAtImbalance{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Below, shared.Above, shared.Below},
			Reaction:      shared.Chop,
			CreatedOn:     asiaSessionTime,
			Status:        make(chan shared.StatusCode, 1),
		},
		Imbalance: &shared.Imbalance{
			Market:      market,
			High:        float64(10),
			Low:         float64(4),
			Midpoint:    float64(7),
			Timeframe:   shared.FiveMinute,
			Sentiment:   shared.Bullish,
			GapRatio:    float64(0.8),
			Purged:      *atomic.NewBool(false),
			Invalidated: *atomic.NewBool(false),
			Date:        asiaSessionTime,
		},
	}

	// Ensure the engine can handle an imbalance break reaction signal.
	eng.handleReactionAtImbalance(&chopImbalanceReaction)
	<-chopImbalanceReaction.Status
}

func generateSessionTimes(t *testing.T) (time.Time, time.Time) {
	now, loc, err := shared.NewYorkTime()
	assert.NoError(t, err)

	asiaSessionStr := "18:30"
	asiaSession, err := time.Parse(shared.SessionTimeLayout, asiaSessionStr)
	assert.NoError(t, err)
	asiaSessionTime := time.Date(now.Year(), now.Month(), now.Day(), asiaSession.Hour(), asiaSession.Minute(), 0, 0, loc)

	londonSessionStr := "9:00" // within high volume window
	londonSession, err := time.Parse(shared.SessionTimeLayout, londonSessionStr)
	assert.NoError(t, err)
	londonSessionTime := time.Date(now.Year(), now.Month(), now.Day(), londonSession.Hour(), londonSession.Minute(), 0, 0, loc)

	return asiaSessionTime, londonSessionTime
}

func TestEstimateStopLoss(t *testing.T) {
	avgVolume := float64(10)
	asianSessionTime, _ := generateSessionTimes(t)
	bullishCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(3),
			Engulfing: false,
			High:      7,
			Low:       3,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.High,
			Volume:    float64(6),
			Engulfing: true,
			High:      9,
			Low:       2,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      12,
			Low:       5,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      16,
			Low:       8,
			Date:      asianSessionTime,
		},
	}

	bearishCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(3),
			Engulfing: false,
			High:      9,
			Low:       7,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Marubozu,
			Sentiment: shared.Bearish,
			Momentum:  shared.High,
			Volume:    float64(6),
			Engulfing: true,
			High:      10,
			Low:       6,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      9,
			Low:       5,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Pinbar,
			Sentiment: shared.Bearish,
			Momentum:  shared.Medium,
			Volume:    float64(4),
			Engulfing: false,
			High:      7,
			Low:       4,
			Date:      asianSessionTime,
		},
	}

	noSignalCandleMeta := []*shared.CandleMetadata{
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(2),
			Engulfing: false,
			High:      9,
			Low:       7,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(2),
			Engulfing: false,
			High:      11,
			Low:       9,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(2),
			Engulfing: false,
			High:      13,
			Low:       11,
			Date:      asianSessionTime,
		},
		{
			Kind:      shared.Doji,
			Sentiment: shared.Bullish,
			Momentum:  shared.Medium,
			Volume:    float64(2),
			Engulfing: false,
			High:      16,
			Low:       13,
			Date:      asianSessionTime,
		},
	}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, []*shared.CandleMetadata{}, &marketSkew)

	market := "^GSPC"
	supportLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asianSessionTime,
			CurrentPrice:  float64(16),
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(5),
			Kind:   shared.Support,
		},
	}

	resistanceLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Below, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Reversal,
			CreatedOn:     asianSessionTime,
			CurrentPrice:  float64(4),
			Status:        make(chan shared.StatusCode, 1),
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(9),
			Kind:   shared.Resistance,
		},
	}

	tests := []struct {
		name          string
		meta          []*shared.CandleMetadata
		levelReaction *shared.ReactionAtLevel
		wantErr       bool
		stoploss      float64
		pointsRange   float64
	}{
		{
			"no candle metadata provided",
			[]*shared.CandleMetadata{},
			supportLevelReaction,
			true,
			0.0,
			0.0,
		},
		{
			"support reversal stop loss",
			bullishCandleMeta,
			supportLevelReaction,
			false,
			1.0,
			15.0,
		},
		{
			"resistance reversal stop loss",
			bearishCandleMeta,
			resistanceLevelReaction,
			false,
			11.0,
			7.0,
		},
		{
			"no signal candle",
			noSignalCandleMeta,
			supportLevelReaction,
			false,
			6.0,
			10.0,
		},
	}

	for _, test := range tests {
		sl, pr, err := eng.estimateStopLoss(&test.levelReaction.ReactionAtFocus, test.meta)
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
	levelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asianSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
	}

	// Ensure confluence points are not awarded for asian session.
	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}
	err := eng.evaluateHighVolumeSession(&levelReaction.ReactionAtFocus, &confluence, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(0))
	assert.Equal(t, len(reasons), 0)

	// Ensure confluence points are awarded for times within the high volume  window (london & new york)
	levelReaction.CreatedOn = londonSessionTime

	err = eng.evaluateHighVolumeSession(&levelReaction.ReactionAtFocus, &confluence, reasons)
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

func TestEvaluatePriceReversalAtLevelConfirmation(t *testing.T) {
	avgVolume := float64(10)
	candleMeta := []*shared.CandleMetadata{}
	marketSkew := shared.NeutralSkew
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	asianSessionTime, _ := generateSessionTimes(t)

	confluence := uint32(0)
	reasons := map[shared.Reason]struct{}{}
	sentiment := shared.Neutral
	market := "^GSPC"
	supportLevelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asianSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
	}

	resistanceLevelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Below, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Reversal,
			CreatedOn:     asianSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Resistance,
		},
	}

	// Ensure bullish price reactions can be confirmed.
	err := eng.evaluatePriceReversalConfirmation(&supportLevelReaction.ReactionAtFocus, &confluence, &sentiment, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, sentiment, shared.Bullish)

	slice := extractReasons(reasons)

	assert.Equal(t, slice[0], shared.ReversalAtSupport)

	// Ensure bearish price reactions can be confirmed.
	confluence = 0
	reasons = map[shared.Reason]struct{}{}
	sentiment = shared.Neutral
	err = eng.evaluatePriceReversalConfirmation(&resistanceLevelReaction.ReactionAtFocus, &confluence, &sentiment, reasons)
	assert.NoError(t, err)
	assert.Equal(t, confluence, uint32(1))
	assert.Equal(t, sentiment, shared.Bearish)

	slice = extractReasons(reasons)

	assert.Equal(t, slice[0], shared.ReversalAtResistance)

	// Ensure the reversal confirmation errors if the level reaction is not a reversal.
	invalidReversalLevelReaction := shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Break,
			CreatedOn:     asianSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(4),
			Kind:   shared.Support,
		},
	}

	err = eng.evaluatePriceReversalConfirmation(&invalidReversalLevelReaction.ReactionAtFocus, &confluence, &sentiment, reasons)
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
	timeframe := shared.FiveMinute
	eng, _, _ := setupEngine(&avgVolume, candleMeta, &marketSkew)

	// Ensure average volume requests can be processed.
	market := "^GSPC"
	avgVol, err := eng.fetchAverageVolume(market, timeframe)
	assert.NoError(t, err)
	assert.Equal(t, avgVol, float64(10))
}

func TestFetchCandleMetadata(t *testing.T) {
	avgVolume := float64(10)
	timeframe := shared.FiveMinute
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
	meta, err := eng.fetchCandleMetadata(market, timeframe)
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

func TestEvaluatePriceReversalAtLevel(t *testing.T) {
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
	levelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(2),
			Kind:   shared.Support,
		},
	}

	// Ensure price reversal is not evaluated if the meta is an empty slice.
	signal, _, _, err := eng.evaluatePriceReversal(&levelReaction.ReactionAtFocus, []*shared.CandleMetadata{}, minLevelReversalConfluence)
	assert.Error(t, err)

	// Ensure price reversal is evualuated as expected with valid input.
	signal, confluence, reasons, err := eng.evaluatePriceReversal(&levelReaction.ReactionAtFocus, candleMeta, minLevelReversalConfluence)
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
	levelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			CurrentPrice:  float64(18),
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(5),
			Kind:   shared.Resistance,
		},
	}

	// Ensure price break is not evaluated if the meta is an empty slice.
	signal, _, _, err := eng.evaluateLevelBreak(&levelReaction.ReactionAtFocus, []*shared.CandleMetadata{}, minLevelBreakConfluence)
	assert.Error(t, err)

	// Ensure price reversal is evualuated as expected with valid input.
	signal, confluence, reasons, err := eng.evaluateLevelBreak(&levelReaction.ReactionAtFocus, candleMeta, minLevelBreakConfluence)
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
	supportLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			LevelKind:     shared.Support,
			CurrentPrice:  float64(14),
			Timeframe:     shared.FiveMinute,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(3),
			Kind:   shared.Support,
		},
	}

	resistanceLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			CurrentPrice:  float64(1),
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Below, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Reversal,
			CreatedOn:     asiaSessionTime,
		},
		Level: &shared.Level{
			Market: market,
			Price:  float64(10),
			Kind:   shared.Resistance,
		},
	}

	// Ensure a support price reversal triggers a long entry signal for a market long or neutral skewed.
	err := eng.evaluatePriceReversalStrength(&supportLevelReaction.ReactionAtFocus, candleMeta, minLevelReversalConfluence)
	assert.NoError(t, err)
	entrySignal := <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Long)

	// Ensure a support price reversal triggers a short exit signal for a market short skewed.
	marketSkew = shortSkew
	err = eng.evaluatePriceReversalStrength(&supportLevelReaction.ReactionAtFocus, candleMeta, minLevelReversalConfluence)
	assert.NoError(t, err)
	exitSignal := <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Short)

	// Ensure a resistance price reversal triggers a long exit signal for a market long skewed.
	marketSkew = longSkew
	candleMeta = resistanceCandleMeta
	err = eng.evaluatePriceReversalStrength(&resistanceLevelReaction.ReactionAtFocus, candleMeta, minLevelReversalConfluence)
	assert.NoError(t, err)
	exitSignal = <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Long)

	// Ensure a resistance price reversal triggers a short entry signal for a market short or neutral skewed.
	marketSkew = shortSkew
	candleMeta = resistanceCandleMeta
	err = eng.evaluatePriceReversalStrength(&resistanceLevelReaction.ReactionAtFocus, candleMeta, minLevelReversalConfluence)
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
	supportLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			CurrentPrice:  float64(1),
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Support,
			PriceMovement: []shared.PriceMovement{shared.Below, shared.Below, shared.Below, shared.Below},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
		},
	}

	resistanceLevelReaction := &shared.ReactionAtLevel{
		ReactionAtFocus: shared.ReactionAtFocus{
			Market:        market,
			CurrentPrice:  float64(18),
			Timeframe:     shared.FiveMinute,
			LevelKind:     shared.Resistance,
			PriceMovement: []shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above},
			Reaction:      shared.Break,
			CreatedOn:     asiaSessionTime,
		},
	}

	// Ensure a support price break triggers a short entry signal for a market short or neutral skewed.
	err := eng.evaluateBreakStrength(&supportLevelReaction.ReactionAtFocus, candleMeta, minLevelBreakConfluence)
	assert.NoError(t, err)
	entrySignal := <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Short)

	// Ensure a support price break triggers a short exit signal for a market long skewed.
	marketSkew = longSkew
	err = eng.evaluateBreakStrength(&supportLevelReaction.ReactionAtFocus, candleMeta, minLevelBreakConfluence)
	assert.NoError(t, err)
	exitSignal := <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Long)

	// Ensure a resistance level break triggers a long entry signal for a market long skewed.
	candleMeta = resistanceBreakCandleMeta
	err = eng.evaluateBreakStrength(&resistanceLevelReaction.ReactionAtFocus, candleMeta, minLevelBreakConfluence)
	assert.NoError(t, err)
	entrySignal = <-entrySignals
	assert.Equal(t, entrySignal.Direction, shared.Long)

	// Ensure a resistance level break triggers a short exit signal for a market short skewed.
	marketSkew = shortSkew
	err = eng.evaluateBreakStrength(&resistanceLevelReaction.ReactionAtFocus, candleMeta, minLevelBreakConfluence)
	assert.NoError(t, err)
	exitSignal = <-exitSignals
	assert.Equal(t, exitSignal.Direction, shared.Short)
}
