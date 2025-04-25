package priceaction

import (
	"context"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupManager(t *testing.T, market string) *Manager {
	subs := make([]chan shared.Candlestick, 0, 10)
	subscribe := func(sub chan shared.Candlestick) {
		subs = append(subs, sub)
	}

	requestPriceData := func(req shared.PriceDataRequest) {
		data := []*shared.Candlestick{}
		for idx := range 4 {
			candle := shared.Candlestick{
				Open:   float64(idx),
				Close:  float64(idx),
				High:   float64(idx),
				Low:    float64(idx),
				Volume: float64(idx),

				Market:    req.Market,
				Timeframe: shared.FiveMinute,
			}

			data = append(data, &candle)
		}

		go func() { req.Response <- data }()
	}
	levelReactionSignals := make(chan shared.LevelReaction, 5)
	signalLevelReaction := func(reaction shared.LevelReaction) {
		levelReactionSignals <- reaction
	}
	cfg := &ManagerConfig{
		Markets:             []string{market},
		Subscribe:           subscribe,
		RequestPriceData:    requestPriceData,
		SignalLevelReaction: signalLevelReaction,
		Logger:              &log.Logger,
	}

	mgr, err := NewManager(cfg)
	assert.NoError(t, err)

	return mgr
}

func TestManager(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure the price action manager can be started.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure the price action manager can receive market candlestick updates.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	mgr.SendMarketUpdate(firstCandle)

	// Ensure the price action manager can receive level signals.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  20,
	}

	mgr.SendLevelSignal(levelSignal)

	// Ensure the price action manager can process candle metadata requests.
	candleMetaReq := shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata),
	}

	mgr.SendCandleMetadataRequest(candleMetaReq)
	<-candleMetaReq.Response

	// Ensure the price action manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestManagerHandleUpdateSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure handling update signals for an unknown market errors.
	wrongMarketCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
	}

	err := mgr.handleUpdateSignal(&wrongMarketCandle)
	assert.Error(t, err)

	// Ensure handling update signals for a valid market succeeds.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	err = mgr.handleUpdateSignal(&firstCandle)
	assert.NoError(t, err)
	assert.NotNil(t, mgr.markets[market].FetchCurrentCandle())

	// Add a level for reaction tests.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  3,
	}
	err = mgr.handleLevelSignal(levelSignal)
	assert.NoError(t, err)

	// Trigger a price request for the market.
	mgr.markets[market].requestingPriceData.Store(true)

	// Ensure handling update signals when a price request is flagged processes the price request.
	secondCandle := shared.Candlestick{
		Open:   float64(10),
		Close:  float64(15),
		High:   float64(20),
		Low:    float64(9),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	err = mgr.handleUpdateSignal(&secondCandle)
	assert.NoError(t, err)

	// Ensure price request flag is reset after the request is processed.
	assert.False(t, mgr.markets[market].requestingPriceData.Load())
}

func TestFillManagerChannels(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  5,
	}

	metaRequest := shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendLevelSignal(levelSignal)
		mgr.SendMarketUpdate(candle)
		mgr.SendCandleMetadataRequest(metaRequest)
	}

	assert.Equal(t, len(mgr.metaSignals), bufferSize)
	assert.Equal(t, len(mgr.updateSignals), bufferSize)
	assert.Equal(t, len(mgr.levelSignals), bufferSize)
}

func TestManagerHandleLevelSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Add some market data.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	err := mgr.handleUpdateSignal(&firstCandle)
	assert.NoError(t, err)

	// Ensure handling level signals from an unknown market returns an error.
	wrongMarketLevelSignal := shared.LevelSignal{
		Market: "^AAPL",
		Price:  20,
	}
	err = mgr.handleLevelSignal(wrongMarketLevelSignal)
	assert.Error(t, err)

	// Ensure handling level signals from a valid market is processed as expected.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  20,
	}
	err = mgr.handleLevelSignal(levelSignal)
	assert.NoError(t, err)
}

func TestManagerHandleCandleMetadataSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure requesting candle metadate for an unknown market errors.
	req := shared.CandleMetadataRequest{
		Market:   "^AAPL",
		Response: make(chan []*shared.CandleMetadata),
	}

	err := mgr.handleCandleMetadataRequest(&req)
	assert.Error(t, err)

	// Generate candles to update the market.
	for idx := range 5 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),

			Market:    market,
			Timeframe: shared.FiveMinute,
		}

		err := mgr.handleUpdateSignal(&candle)
		assert.NoError(t, err)

	}

	// Ensure requesting candle metadate for a valid market succeeds.
	req = shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata),
	}

	go func() {
		<-req.Response
	}()

	err = mgr.handleCandleMetadataRequest(&req)
	assert.NoError(t, err)
}
