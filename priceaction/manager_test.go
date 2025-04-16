package priceaction

import (
	"context"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestManager(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	subs := make([]chan shared.Candlestick, 0, 10)
	subscribe := func(sub chan shared.Candlestick) {
		subs = append(subs, sub)
	}

	priceDataReqs := make(chan shared.PriceDataRequest, 5)
	requestPriceData := func(req shared.PriceDataRequest) {
		priceDataReqs <- req
	}
	levelReactionSignals := make(chan shared.LevelReaction, 5)
	signalLevelReaction := func(reaction shared.LevelReaction) {
		levelReactionSignals <- reaction
	}
	cfg := &ManagerConfig{
		MarketIDs:           []string{market},
		Subscribe:           subscribe,
		RequestPriceData:    requestPriceData,
		SignalLevelReaction: signalLevelReaction,
		Logger:              &log.Logger,
	}

	mgr, err := NewManager(cfg)
	assert.NoError(t, err)

	// Ensure the price action manager can be started.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure the price action manager can price market candlestick updates.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Done:      make(chan struct{}),
	}

	mgr.SendMarketUpdate(firstCandle)
	<-firstCandle.Done

	secondCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Done:      make(chan struct{}),
	}

	mgr.SendMarketUpdate(secondCandle)
	<-firstCandle.Done

	// Ensure processing a level signal for an unknown market is a no-op.
	levelSignal := shared.LevelSignal{
		Market: "unknown",
		Price:  20,
		Done:   make(chan struct{}),
	}

	mgr.SendLevelSignal(levelSignal)
	<-levelSignal.Done

	// Ensure the price action manager can process level signals.
	levelSignal = shared.LevelSignal{
		Market: market,
		Price:  20,
		Done:   make(chan struct{}),
	}

	mgr.SendLevelSignal(levelSignal)
	<-levelSignal.Done

	// Ensure the price action manager can process candle metadata requests.
	candleMetaReq := shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan shared.CandleMetadata),
	}

	mgr.SendCandleMetadataRequest(candleMetaReq)
	<-candleMetaReq.Response

	// todo: expand test cases.

	// Ensure the price action manager can be gracefully shutdown.
	cancel()
	<-done
}
