package engine

import (
	"context"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupEngine() (*Engine, chan shared.AverageVolumeRequest, chan shared.CandleMetadataRequest) {
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

	marketStatusReqs := make(chan shared.MarketStatusRequest, 5)
	requestMarketStatus := func(req shared.MarketStatusRequest) {
		marketStatusReqs <- req
	}

	cfg := &EngineConfig{
		RequestCandleMetadata: requestCandleMetadata,
		RequestAverageVolume:  requestAvgVolume,
		SendEntrySignal:       signalEntry,
		SendExitSignal:        signalExit,
		RequestMatketStatus:   requestMarketStatus,
		Logger:                log.Logger,
	}

	eng := NewEngine(cfg)

	return eng, avgVolumeReqs, candleMetadataReqs
}
func TestEngine(t *testing.T) {
	eng, _, _ := setupEngine()

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
	eng, _, _ := setupEngine()

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

func TestEstimateStopLoss(t *testing.T) {
	eng, _, _ := setupEngine()

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

func TestEvaluateReversal(t *testing.T) {
	eng, avgVolumeReqs, _ := setupEngine()

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	market := "^GSPC"
	avgVol := float64(4)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-avgVolumeReqs:
				req.Response <- avgVol
			}
		}
	}()

	tests := []struct {
		name        string
		market      string
		candleMeta  []*shared.CandleMetadata
		sentiment   shared.Sentiment
		signal      bool
		confluences float64
		reasons     []shared.Reason
		wantErr     bool
	}{
		{
			"high confluence bullish reversal at support",
			market,
			[]*shared.CandleMetadata{
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bearish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      5,
					Low:       4,
					Date:      now,
				},
				{
					Kind:      shared.Pinbar,
					Sentiment: shared.Bullish,
					Momentum:  shared.Medium,
					Volume:    float64(4),
					Engulfing: false,
					High:      6,
					Low:       2,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bullish,
					Momentum:  shared.Medium,
					Volume:    float64(5),
					Engulfing: false,
					High:      6,
					Low:       2,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bullish,
					Momentum:  shared.High,
					Volume:    float64(8),
					Engulfing: false,
					High:      14,
					Low:       6,
					Date:      now,
				},
			},
			shared.Bullish,
			true,
			7,
			[]shared.Reason{shared.ReversalAtSupport, shared.StrongMove, shared.StrongVolume},
			false,
		},
		{
			"high confluence bearish reversal at resistance",
			market,
			[]*shared.CandleMetadata{
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bullish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      10,
					Low:       9,
					Date:      now,
				},
				{
					Kind:      shared.Pinbar,
					Sentiment: shared.Bearish,
					Momentum:  shared.Medium,
					Volume:    float64(4),
					Engulfing: false,
					High:      11,
					Low:       7,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bearish,
					Momentum:  shared.Medium,
					Volume:    float64(5),
					Engulfing: false,
					High:      8,
					Low:       6,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bearish,
					Momentum:  shared.High,
					Volume:    float64(8),
					Engulfing: false,
					High:      6,
					Low:       1,
					Date:      now,
				},
			},
			shared.Bearish,
			true,
			7,
			[]shared.Reason{shared.ReversalAtResistance, shared.StrongMove, shared.StrongVolume},
			false,
		},
		{
			"low volume chop at support",
			market,
			[]*shared.CandleMetadata{
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bullish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      12,
					Low:       8,
					Date:      now,
				},
				{
					Kind:      shared.Pinbar,
					Sentiment: shared.Bearish,
					Momentum:  shared.Medium,
					Volume:    float64(2),
					Engulfing: false,
					High:      13,
					Low:       9,
					Date:      now,
				},
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bullish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      10,
					Low:       9,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bullish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      12,
					Low:       10,
					Date:      now,
				},
			},
			shared.Bearish,
			false,
			1,
			[]shared.Reason{shared.StrongVolume},
			false,
		},
		{
			"low volume chop at resistance",
			market,
			[]*shared.CandleMetadata{
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bearish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      12,
					Low:       8,
					Date:      now,
				},
				{
					Kind:      shared.Pinbar,
					Sentiment: shared.Bullish,
					Momentum:  shared.Medium,
					Volume:    float64(2),
					Engulfing: false,
					High:      12,
					Low:       7,
					Date:      now,
				},
				{
					Kind:      shared.Doji,
					Sentiment: shared.Bearish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      10,
					Low:       9,
					Date:      now,
				},
				{
					Kind:      shared.Marubozu,
					Sentiment: shared.Bullish,
					Momentum:  shared.Low,
					Volume:    float64(1),
					Engulfing: false,
					High:      12,
					Low:       10,
					Date:      now,
				},
			},
			shared.Bearish,
			false,
			0,
			[]shared.Reason{},
			false,
		},
		{
			"empty candle meta",
			market,
			[]*shared.CandleMetadata{},
			shared.Bearish,
			false,
			0,
			[]shared.Reason{},
			true,
		},
	}

	for _, test := range tests {
		signal, confluences, reasons, err := eng.evaluateReversal(test.market, test.candleMeta, test.sentiment)
		if test.wantErr && err == nil {
			t.Errorf("%s: expected an error, got none", test.name)
		}

		if !test.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		if !test.wantErr {
			if signal != test.signal {
				t.Errorf("signal mismatch, expected (%v), got (%v)", test.signal, signal)
			}

			if confluences != uint32(test.confluences) {
				t.Errorf("signal mismatch, expected (%v), got (%v)", test.confluences, confluences)
			}

			if len(reasons) != len(test.reasons) {
				t.Errorf("reasons mismatch, expected (%v), got (%v)", test.reasons, reasons)
			}
		}
	}

	cancel()
}
