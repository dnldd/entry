package fetch

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestManager(t *testing.T) {
	// Ensure the fetch manager can be created.
	fmpCfg := &FMPConfig{
		APIKey:  "key",
		BaseURL: "http://base",
	}

	fmp := NewFMPClient(fmpCfg)

	caughtUpSignals := make(chan shared.CaughtUpSignal, 5)
	signalCaughtUp := func(signal shared.CaughtUpSignal) {
		caughtUpSignals <- signal
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &ManagerConfig{
		ExchangeClient: fmp,
		SignalCaughtUp: signalCaughtUp,
		JobScheduler:   gocron.NewScheduler(loc),
		Logger:         &log.Logger,
	}

	mgr, err := NewManager(cfg)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure the fetch manager can be run.
	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure entities can subscribe for market updates.
	sub := make(chan shared.Candlestick, 5)
	mgr.Subscribe(sub)

	// Ensure subscribers are notified of market updates.
	candle := &shared.Candlestick{
		Open:   float64(6),
		Close:  float64(9),
		High:   float64(10),
		Low:    float64(4),
		Volume: float64(3),
	}

	mgr.notifySubscribers(candle)
	notifiedCandle := <-sub
	assert.Equal(t, *candle, notifiedCandle)

	// Ensure the manage can process catch up signals.
	market := "^GSPC"
	catchUp := shared.CatchUpSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Start:     time.Time{},
		Done:      make(chan struct{}),
	}

	mgr.SendCatchUpSignal(catchUp)
	<-catchUp.Done

	// Ensure the fetch manager can be gracefully terminated.
	cancel()
	<-done
}
