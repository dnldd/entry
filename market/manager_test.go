package market

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupManager(t *testing.T, market string, now time.Time, backtest bool) (*Manager, chan shared.CatchUpSignal, chan shared.LevelSignal) {
	bufferSize := 10
	subscriptions := make([]chan shared.Candlestick, 0, bufferSize)
	subscribe := func(name string, sub chan shared.Candlestick) {
		subscriptions = append(subscriptions, sub)
	}

	catchUpSignals := make(chan shared.CatchUpSignal, bufferSize)
	catchUp := func(signal shared.CatchUpSignal) {
		signal.Status <- shared.Processed
		catchUpSignals <- signal
	}

	signalLevelSignals := make(chan shared.LevelSignal, bufferSize)
	signalLevel := func(signal shared.LevelSignal) {
		signal.Status <- shared.Processed
		signalLevelSignals <- signal
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &ManagerConfig{
		Markets:      []string{market},
		Subscribe:    subscribe,
		CatchUp:      catchUp,
		SignalLevel:  signalLevel,
		Backtest:     backtest,
		JobScheduler: gocron.NewScheduler(loc),
		Logger:       &log.Logger,
	}

	mgr, err := NewManager(cfg, now)
	assert.NoError(t, err)

	return mgr, catchUpSignals, signalLevelSignals
}

func TestManager(t *testing.T) {
	// Ensure the market manager can be started.
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, catchUpSignals, _ := setupManager(t, market, now, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure running the manager triggers a catch up signal for tracked markets.
	sig := <-catchUpSignals
	assert.Equal(t, sig.Market, market)

	// Ensure the manager can handle a catch up signal.
	signal := shared.NewCaughtUpSignal(market)
	mgr.SendCaughtUpSignal(signal)

	// Ensure the manager can process a market update.
	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	mgr.SendMarketUpdate(candle)

	// Ensure the manager can process a price data request.
	priceDataReq := shared.PriceDataRequest{
		Market:   market,
		Response: make(chan []*shared.Candlestick, 5),
	}

	mgr.SendPriceDataRequest(priceDataReq)
	<-priceDataReq.Response

	// Ensure the manager can process an average volume request.
	avgVolumeReq := shared.AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64, 5),
	}

	mgr.SendAverageVolumeRequest(avgVolumeReq)
	avgVol := <-avgVolumeReq.Response
	assert.Equal(t, avgVol, float64(2))

	// Ensure the manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	caughtUpSignal := shared.CaughtUpSignal{
		Market: market,
	}

	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	priceDataReq := shared.PriceDataRequest{
		Market:   market,
		Response: make(chan []*shared.Candlestick, 5),
	}

	avgVolumeReq := shared.AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64, 5),
	}

	vwapDataReq := shared.VWAPDataRequest{
		Market:   market,
		Response: make(chan []*shared.VWAP, 5),
	}

	vwapReq := shared.VWAPRequest{
		Market:   market,
		At:       time.Time{},
		Response: make(chan *shared.VWAP, 1),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendAverageVolumeRequest(avgVolumeReq)
		mgr.SendCaughtUpSignal(caughtUpSignal)
		mgr.SendMarketUpdate(candle)
		mgr.SendPriceDataRequest(priceDataReq)
		mgr.SendVWAPDataRequest(vwapDataReq)
		mgr.SendVWAPRequest(vwapReq)
	}

	assert.Equal(t, len(mgr.averageVolumeRequests), bufferSize)
	assert.Equal(t, len(mgr.caughtUpSignals), bufferSize)
	assert.Equal(t, len(mgr.updateSignals), bufferSize)
	assert.Equal(t, len(mgr.priceDataRequests), bufferSize)
	assert.Equal(t, len(mgr.vwapDataRequests), bufferSize)
	assert.Equal(t, len(mgr.vwapRequests), bufferSize)
}

func TestHandleUpdateCandle(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Ensure processing a candle with an unknown market errors.
	wrongMarketCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleUpdateCandle(&wrongMarketCandle)
	assert.Error(t, err)

	// Ensure processing a valid candle succeeds as expected.
	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleUpdateCandle(&candle)
	assert.NoError(t, err)
}

func TestHandleCaughtUpSignal(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Ensure processing a caught up signal for an unknown market errors.
	wrongMarketCaughtUpSignal := shared.CaughtUpSignal{
		Market: "^AAPL",
		Status: make(chan shared.StatusCode, 1),
	}

	err = mgr.handleCaughtUpSignal(&wrongMarketCaughtUpSignal)
	assert.Error(t, err)

	// Ensure processing a valid caught up signal succeeds as expected.
	caughtUpSignal := shared.CaughtUpSignal{
		Market: market,
		Status: make(chan shared.StatusCode, 1),
	}

	err = mgr.handleCaughtUpSignal(&caughtUpSignal)
	assert.NoError(t, err)

	// Ensure a market's caught up state can be fetched.
	caughtUp, err := mgr.FetchCaughtUpState(market)
	assert.NoError(t, err)
	assert.True(t, caughtUp)
}

func TestHandleAverageVolumeSignal(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Update the market with a candle.
	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleUpdateCandle(&candle)
	assert.NoError(t, err)

	// Ensure requesting an average volume for an unknown market errors.
	unknownMarketAvgVolumeReq := shared.AverageVolumeRequest{
		Market:   "^AAPL",
		Response: make(chan float64, 5),
	}

	err = mgr.handleAverageVolumeRequest(&unknownMarketAvgVolumeReq)
	assert.Error(t, err)

	// Ensure requesting a valid market average volume succeeds.
	avgVolumeReq := shared.AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64, 5),
	}

	err = mgr.handleAverageVolumeRequest(&avgVolumeReq)
	assert.NoError(t, err)
	resp := <-avgVolumeReq.Response
	assert.Equal(t, resp, candle.Volume)

	// Ensure subsequent average volume request use the cache.
	err = mgr.handleAverageVolumeRequest(&avgVolumeReq)
	assert.NoError(t, err)
	resp = <-avgVolumeReq.Response
	assert.Equal(t, resp, candle.Volume)
}

func TestHandlePriceDataRequest(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Update the market with candle data.

	for idx := range 6 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),
			Date:   now,

			Market:    market,
			Timeframe: shared.FiveMinute,
			Status:    make(chan shared.StatusCode, 1),
		}

		err = mgr.handleUpdateCandle(&candle)
		assert.NoError(t, err)
	}

	mgr.marketsMtx.RLock()
	mkt := mgr.markets[market]
	mgr.marketsMtx.RUnlock()

	// Mark the market as caught up.
	mkt.caughtUp.Store(true)

	// Ensure a price data request for an unknown market errors.
	unknownPriceDataReq := shared.PriceDataRequest{
		Market:   "^AAPL",
		Response: make(chan []*shared.Candlestick, 5),
	}

	err = mgr.handlePriceDataRequest(&unknownPriceDataReq)
	assert.Error(t, err)

	// Ensure a valid price data request succeeds.
	priceDataReq := shared.PriceDataRequest{
		Market:   market,
		Response: make(chan []*shared.Candlestick, 5),
	}

	err = mgr.handlePriceDataRequest(&priceDataReq)
	assert.NoError(t, err)
	req := <-priceDataReq.Response
	assert.GreaterThan(t, len(req), 0)
}

func TestHandleVWAPDataRequest(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Update the market with candle data.
	for idx := range 6 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),
			Date:   now,

			Market:    market,
			Timeframe: shared.FiveMinute,
			Status:    make(chan shared.StatusCode, 1),
		}

		err = mgr.handleUpdateCandle(&candle)
		assert.NoError(t, err)
	}

	mgr.marketsMtx.RLock()
	mkt := mgr.markets[market]
	mgr.marketsMtx.RUnlock()

	// Mark the market as caught up.
	mkt.caughtUp.Store(true)

	// Ensure a vwap data request for an unknown market errors.
	unknownVWAPDataReq := shared.VWAPDataRequest{
		Market:   "^AAPL",
		Response: make(chan []*shared.VWAP, 5),
	}

	err = mgr.handleVWAPDataRequest(&unknownVWAPDataReq)
	assert.Error(t, err)

	// Ensure a valid vwap data request succeeds.
	vwapDataReq := shared.VWAPDataRequest{
		Market:   market,
		Response: make(chan []*shared.VWAP, 5),
	}

	err = mgr.handleVWAPDataRequest(&vwapDataReq)
	assert.NoError(t, err)
	req := <-vwapDataReq.Response
	assert.GreaterThan(t, len(req), 0)
}

func TestHandleVWAPRequest(t *testing.T) {
	market := "^GSPC"

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, _, _ := setupManager(t, market, now, false)

	// Update the market with candle data.
	for idx := range 6 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),
			Date:   now,

			Market:    market,
			Timeframe: shared.FiveMinute,
			Status:    make(chan shared.StatusCode, 1),
		}

		err = mgr.handleUpdateCandle(&candle)
		assert.NoError(t, err)
	}

	mgr.marketsMtx.RLock()
	mkt := mgr.markets[market]
	mgr.marketsMtx.RUnlock()

	// Mark the market as caught up.
	mkt.caughtUp.Store(true)

	// Ensure a vwap request for an unknown market errors.
	unknownVWAPReq := shared.VWAPRequest{
		Market:   "^AAPL",
		At:       now,
		Response: make(chan *shared.VWAP, 1),
	}

	err = mgr.handleVWAPRequest(&unknownVWAPReq)
	assert.Error(t, err)

	// Ensure a valid vwap request succeeds.
	vwapReq := shared.VWAPRequest{
		Market:   market,
		At:       now,
		Response: make(chan *shared.VWAP, 1),
	}

	err = mgr.handleVWAPRequest(&vwapReq)
	assert.NoError(t, err)
	vwap := <-vwapReq.Response
	assert.Equal(t, vwap.Value, float64(3.6666666666666665))
}

func TestBacktestLevelGeneration(t *testing.T) {
	market := "^GSPC"
	backtest := true

	// Ensure the market manager starts at the time of the historic data.
	startTimeStr := "2025-05-01 02:45:00"
	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	start, err := time.ParseInLocation(shared.DateLayout, startTimeStr, loc)
	assert.NoError(t, err)

	mgr, _, levelSignals := setupManager(t, market, start, backtest)

	notifySubscribersFunc := func(candle shared.Candlestick) error {
		mgr.SendMarketUpdate(candle)
		return nil
	}

	hCfg := &shared.HistoricDataConfig{
		Market:            market,
		Timeframe:         shared.FiveMinute,
		FilePath:          "../testdata/historicdata.json",
		SignalCaughtUp:    mgr.SendCaughtUpSignal,
		NotifySubscribers: notifySubscribersFunc,
		Logger:            &log.Logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedLevelPrices := []float64{36, 18}
	runDone := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(runDone)
				return
			case sig := <-levelSignals:
				// Ensure the historical data source triggers level signals as expected.
				assert.In(t, sig.Price, expectedLevelPrices)
			}
		}
	}()

	historicData, err := shared.NewHistoricData(hCfg)
	assert.NoError(t, err)

	mgrDone := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(mgrDone)
	}()

	err = historicData.ProcessHistoricalData()
	assert.NoError(t, err)

	cancel()
	<-runDone
	<-mgrDone
}
