package market

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setupManager(t *testing.T, market string, now time.Time, backtest bool) (*Manager, chan shared.CatchUpSignal, chan shared.LevelSignal) {
	bufferSize := 200
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

	relayMarketUpdateSignals := make(chan shared.Candlestick, bufferSize)
	relayMarketUpdate := func(candle shared.Candlestick) {
		candle.Status <- shared.Processed
		relayMarketUpdateSignals <- candle
	}

	imbalanceSignals := make(chan shared.ImbalanceSignal, 2)
	signalImbalance := func(signal shared.ImbalanceSignal) {
		imbalanceSignals <- signal
		signal.Status <- shared.Processed
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &ManagerConfig{
		Markets:           []string{market},
		Timeframes:        []shared.Timeframe{shared.OneMinute, shared.FiveMinute, shared.OneHour},
		Subscribe:         subscribe,
		CatchUp:           catchUp,
		SignalLevel:       signalLevel,
		SignalImbalance:   signalImbalance,
		RelayMarketUpdate: relayMarketUpdate,
		Backtest:          backtest,
		JobScheduler:      gocron.NewScheduler(loc),
		Logger:            &log.Logger,
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
		Market:    market,
		Timeframe: candle.Timeframe,
		N:         1,
		Response:  make(chan []*shared.Candlestick, 5),
	}

	mgr.SendPriceDataRequest(priceDataReq)
	<-priceDataReq.Response

	// Ensure the manager can process an average volume request.
	avgVolumeReq := shared.AverageVolumeRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan float64, 5),
	}

	mgr.SendAverageVolumeRequest(avgVolumeReq)
	avgVol := <-avgVolumeReq.Response
	assert.Equal(t, avgVol, float64(2))

	// Ensure the manager can process a vwap request.
	vwapReq := shared.VWAPRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		At:        now,
		Response:  make(chan *shared.VWAP, 1),
	}

	mgr.SendVWAPRequest(vwapReq)
	vwap := <-vwapReq.Response
	assert.Equal(t, vwap.Value, 6.666666666666667)

	// Ensure the manager can process a vwap data request.
	vwapDataReq := shared.VWAPDataRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan []*shared.VWAP, 1),
	}

	mgr.SendVWAPDataRequest(vwapDataReq)
	data := <-vwapDataReq.Response
	assert.GreaterThan(t, len(data), 0)

	// Ensure the manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestManagerConfigValidate(t *testing.T) {
	// Helper functions for required fields
	dummySubscribe := func(name string, sub chan shared.Candlestick) {}
	dummyRelayMarketUpdate := func(candle shared.Candlestick) {}
	dummyCatchUp := func(signal shared.CatchUpSignal) {}
	dummySignalLevel := func(signal shared.LevelSignal) {}
	dummySignalImbalance := func(signal shared.ImbalanceSignal) {}

	// Use a real zerolog.Logger and gocron.Scheduler for testing
	logger := zerolog.New(nil)
	scheduler := gocron.NewScheduler(time.UTC)

	baseCfg := &ManagerConfig{
		Markets:           []string{"AAPL"},
		Timeframes:        []shared.Timeframe{shared.OneMinute},
		Subscribe:         dummySubscribe,
		RelayMarketUpdate: dummyRelayMarketUpdate,
		CatchUp:           dummyCatchUp,
		SignalLevel:       dummySignalLevel,
		SignalImbalance:   dummySignalImbalance,
		JobScheduler:      scheduler,
		Logger:            &logger,
	}

	tests := []struct {
		name        string
		modify      func(cfg *ManagerConfig)
		wantErr     bool
		errContains []string
	}{
		{
			name:    "valid config returns nil",
			modify:  func(cfg *ManagerConfig) {},
			wantErr: false,
		},
		{
			name:        "missing Markets",
			modify:      func(cfg *ManagerConfig) { cfg.Markets = nil },
			wantErr:     true,
			errContains: []string{"no markets provided"},
		},
		{
			name:        "missing Timeframes",
			modify:      func(cfg *ManagerConfig) { cfg.Timeframes = nil },
			wantErr:     true,
			errContains: []string{"no timeframes provided"},
		},
		{
			name:        "missing Subscribe",
			modify:      func(cfg *ManagerConfig) { cfg.Subscribe = nil },
			wantErr:     true,
			errContains: []string{"subscribe function cannot be nil"},
		},
		{
			name:        "missing RelayMarketUpdate",
			modify:      func(cfg *ManagerConfig) { cfg.RelayMarketUpdate = nil },
			wantErr:     true,
			errContains: []string{"relay market update function cannot be nil"},
		},
		{
			name:        "missing CatchUp",
			modify:      func(cfg *ManagerConfig) { cfg.CatchUp = nil },
			wantErr:     true,
			errContains: []string{"catch up function cannot be nil"},
		},
		{
			name:        "missing SignalLevel",
			modify:      func(cfg *ManagerConfig) { cfg.SignalLevel = nil },
			wantErr:     true,
			errContains: []string{"signal level function cannot be nil"},
		},
		{
			name:        "missing SignalImbalance",
			modify:      func(cfg *ManagerConfig) { cfg.SignalImbalance = nil },
			wantErr:     true,
			errContains: []string{"signal imbalance function cannot be nil"},
		},
		{
			name:        "missing JobScheduler",
			modify:      func(cfg *ManagerConfig) { cfg.JobScheduler = nil },
			wantErr:     true,
			errContains: []string{"job scheduler cannot be nil"},
		},
		{
			name:        "missing Logger",
			modify:      func(cfg *ManagerConfig) { cfg.Logger = nil },
			wantErr:     true,
			errContains: []string{"logger function cannot be nil"},
		},
		{
			name: "multiple missing fields",
			modify: func(cfg *ManagerConfig) {
				*cfg = ManagerConfig{}
			},
			wantErr: true,
			errContains: []string{
				"no markets provided",
				"no timeframes provided",
				"subscribe function cannot be nil",
				"relay market update function cannot be nil",
				"catch up function cannot be nil",
				"signal level function cannot be nil",
				"signal imbalance function cannot be nil",
				"job scheduler cannot be nil",
				"logger function cannot be nil",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := *baseCfg
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				for _, substr := range tt.errContains {
					assert.True(t, strings.Contains(err.Error(), substr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan []*shared.Candlestick, 5),
	}

	avgVolumeReq := shared.AverageVolumeRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan float64, 5),
	}

	vwapDataReq := shared.VWAPDataRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan []*shared.VWAP, 5),
	}

	vwapReq := shared.VWAPRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		At:        time.Time{},
		Response:  make(chan *shared.VWAP, 1),
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
		Market:    "^AAPL",
		Timeframe: candle.Timeframe,
		Response:  make(chan float64, 5),
	}

	err = mgr.handleAverageVolumeRequest(&unknownMarketAvgVolumeReq)
	assert.Error(t, err)

	// Ensure requesting a valid market average volume succeeds.
	avgVolumeReq := shared.AverageVolumeRequest{
		Market:    market,
		Timeframe: candle.Timeframe,
		Response:  make(chan float64, 5),
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

	timeframe := shared.FiveMinute
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
			Timeframe: timeframe,
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
		Market:    "^AAPL",
		Timeframe: timeframe,
		Response:  make(chan []*shared.Candlestick, 5),
	}

	err = mgr.handlePriceDataRequest(&unknownPriceDataReq)
	assert.Error(t, err)

	// Ensure a valid price data request succeeds.
	priceDataReq := shared.PriceDataRequest{
		Market:    market,
		Timeframe: timeframe,
		N:         6,
		Response:  make(chan []*shared.Candlestick, 5),
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
	timeframe := shared.FiveMinute
	for idx := range 6 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),
			Date:   now,

			Market:    market,
			Timeframe: timeframe,
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
		Market:    "^AAPL",
		Timeframe: timeframe,
		Response:  make(chan []*shared.VWAP, 5),
	}

	err = mgr.handleVWAPDataRequest(&unknownVWAPDataReq)
	assert.Error(t, err)

	// Ensure a valid vwap data request succeeds.
	vwapDataReq := shared.VWAPDataRequest{
		Market:    market,
		Timeframe: timeframe,
		Response:  make(chan []*shared.VWAP, 5),
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
	timeframe := shared.FiveMinute
	for idx := range 6 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),
			Date:   now,

			Market:    market,
			Timeframe: timeframe,
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
		Market:    "^AAPL",
		Timeframe: timeframe,
		At:        now,
		Response:  make(chan *shared.VWAP, 1),
	}

	err = mgr.handleVWAPRequest(&unknownVWAPReq)
	assert.Error(t, err)

	// Ensure a valid vwap request succeeds.
	vwapReq := shared.VWAPRequest{
		Market:    market,
		Timeframe: timeframe,
		At:        now,
		Response:  make(chan *shared.VWAP, 1),
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
		FilePath:          "../testdata/historicdata.json",
		SignalCaughtUp:    mgr.SendCaughtUpSignal,
		NotifySubscribers: notifySubscribersFunc,
		Logger:            &log.Logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedLevelPrices := []float64{36, 18}
	runDone := make(chan struct{})
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
