package market

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

func TestManager(t *testing.T) {
	// Ensure the market manager can be started.
	bufferSize := 2
	subCounter := atomic.NewUint32(0)
	subscriptions := make([]*chan shared.Candlestick, 0, bufferSize)
	subscribe := func(sub *chan shared.Candlestick) {
		subscriptions = append(subscriptions, sub)
		subCounter.Inc()
	}

	catchUpSignals := make(chan shared.CatchUpSignal, bufferSize)
	catchUp := func(signal *shared.CatchUpSignal) {
		catchUpSignals <- *signal
	}

	signalLevelSignals := make(chan shared.LevelSignal, bufferSize)
	signalLevel := func(signal *shared.LevelSignal) {
		signalLevelSignals <- *signal
	}

	market := "^GSPC"

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &ManagerConfig{
		MarketIDs:    []string{market},
		Subscribe:    subscribe,
		CatchUp:      catchUp,
		SignalLevel:  signalLevel,
		JobScheduler: gocron.NewScheduler(loc),
		Logger:       &log.Logger,
	}

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	mgr, err := NewManager(cfg, now)
	assert.NoError(t, err)

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

	// Ensure running the manager triggers a market update subscription for the manager.
	assert.Equal(t, subCounter.Load(), uint32(1))

	mgr.marketsMtx.RLock()
	gspcMarket := mgr.markets[market]
	mgr.marketsMtx.RUnlock()

	// Ensure the manager can handle a catch up signal.
	signal := shared.CaughtUpSignal{
		Market: market,
		Done:   make(chan struct{}),
	}
	mgr.SendCaughtUpSignal(signal)
	<-signal.Done
	assert.True(t, gspcMarket.caughtUp.Load())

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
		Done:      make(chan struct{}),
	}

	mgr.SendMarketUpdate(candle)
	<-candle.Done

	// Ensure the manager can process a price data request.
	priceDataReq := shared.PriceDataRequest{
		Market:   market,
		Response: make(chan []*shared.Candlestick),
	}

	mgr.SendPriceDataRequest(priceDataReq)
	data := <-priceDataReq.Response
	assert.GreaterThan(t, len(data), 0)

	// Ensure the manager can process an average volume request.
	volumeResp := make(chan float64)
	avgVolumeReq := shared.AverageVolumeRequest{
		Market:   market,
		Response: volumeResp,
	}

	mgr.SendAverageVolumeRequest(avgVolumeReq)
	avgVol := <-avgVolumeReq.Response
	assert.Equal(t, avgVol, float64(2))

	// Ensure the manager can be gracefully shutdown.
	cancel()
	<-done
}
