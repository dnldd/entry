package fetch

import (
	"context"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

type FMPMock struct {
	fetchIndexIntradayHistoricalData []gjson.Result
	fetchIndexIntradayHistoricalErr  error
}

func (m *FMPMock) FetchIndexIntradayHistorical(ctx context.Context, market string,
	timeframe shared.Timeframe, start time.Time, end time.Time) ([]gjson.Result, error) {
	return m.fetchIndexIntradayHistoricalData, m.fetchIndexIntradayHistoricalErr
}

func setupManager(t *testing.T) *Manager {
	data := `[{"open":10,"close":12,"high":15,"low":8, "volume":5,"date":"2025-02-04 15:05:00"}]`
	res := gjson.Parse(data).Array()

	fmpMock := FMPMock{
		fetchIndexIntradayHistoricalData: res,
		fetchIndexIntradayHistoricalErr:  nil,
	}

	caughtUpSignals := make(chan shared.CaughtUpSignal, 5)
	signalCaughtUp := func(signal shared.CaughtUpSignal) {
		caughtUpSignals <- signal
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	market := "^GSPC"
	cfg := &ManagerConfig{
		Markets:        []string{market},
		ExchangeClient: &fmpMock,
		SignalCaughtUp: signalCaughtUp,
		JobScheduler:   gocron.NewScheduler(loc),
		Logger:         &log.Logger,
	}

	mgr, err := NewManager(cfg)
	assert.NoError(t, err)

	return mgr
}

func TestManager(t *testing.T) {
	mgr := setupManager(t)

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
	mgr.Subscribe("sub", sub)

	// Ensure subscribers are notified of market updates.
	candle := shared.Candlestick{
		Open:   float64(6),
		Close:  float64(9),
		High:   float64(10),
		Low:    float64(4),
		Volume: float64(3),
	}

	mgr.NotifySubscribers(candle)
	notifiedCandle := <-sub
	assert.Equal(t, candle, notifiedCandle)

	// Ensure the manage can process catch up signals.
	market := "^GSPC"
	catchUp := shared.CatchUpSignal{
		Market:    market,
		Timeframe: []shared.Timeframe{shared.FiveMinute},
		Start:     time.Time{},
		Status:    make(chan shared.StatusCode, 1),
	}

	mgr.SendCatchUpSignal(catchUp)
	<-catchUp.Status

	// Ensure calling a market data job for an unknown market errors.
	err := mgr.fetchMarketDataJob("^AAPL", shared.FiveMinute)
	assert.Error(t, err)

	// Ensure calling a maket data job for a valid market succeeds.
	err = mgr.fetchMarketDataJob(market, shared.FiveMinute)
	assert.NoError(t, err)

	// Ensure the fetch manager can be gracefully terminated.
	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	mgr := setupManager(t)

	market := "^GSPC"
	catchUp := shared.CatchUpSignal{
		Market:    market,
		Timeframe: []shared.Timeframe{shared.FiveMinute},
		Start:     time.Time{},
		Status:    make(chan shared.StatusCode),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendCatchUpSignal(catchUp)
	}

	assert.Equal(t, len(mgr.catchUpSignals), bufferSize)
}

func TestHandleCatchUpSignal(t *testing.T) {
	mgr := setupManager(t)

	// Ensure handling a catch up signal for an unknown market errors.
	unknownMarketCatchUp := shared.CatchUpSignal{
		Market:    "^AAPL",
		Timeframe: []shared.Timeframe{shared.FiveMinute},
		Start:     time.Time{},
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleCatchUpSignal(unknownMarketCatchUp)
	assert.Error(t, err)

	// Ensure handling a valid catch up signal succeeds.
	market := "^GSPC"
	catchUp := shared.CatchUpSignal{
		Market:    market,
		Timeframe: []shared.Timeframe{shared.FiveMinute},
		Start:     time.Time{},
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleCatchUpSignal(catchUp)
	assert.NoError(t, err)
}
