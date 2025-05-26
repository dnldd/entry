package fetch

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

func TestFetchManagerConfigValidate(t *testing.T) {
	// Dummy implementations for required fields
	dummyExchangeClient := new(struct{ shared.MarketFetcher })
	dummySignalCaughtUp := func(signal shared.CaughtUpSignal) {}
	logger := zerolog.New(nil)
	scheduler := gocron.NewScheduler(time.UTC)

	baseCfg := &ManagerConfig{
		Markets:        []string{"AAPL"},
		ExchangeClient: dummyExchangeClient,
		SignalCaughtUp: dummySignalCaughtUp,
		JobScheduler:   scheduler,
		Logger:         &logger,
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
			name:        "missing ExchangeClient",
			modify:      func(cfg *ManagerConfig) { cfg.ExchangeClient = nil },
			wantErr:     true,
			errContains: []string{"exchange client cannot be nil"},
		},
		{
			name:        "missing SignalCaughtUp",
			modify:      func(cfg *ManagerConfig) { cfg.SignalCaughtUp = nil },
			wantErr:     true,
			errContains: []string{"signal caught up function cannot be nil"},
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
			errContains: []string{"logger cannot be nil"},
		},
		{
			name: "multiple missing fields",
			modify: func(cfg *ManagerConfig) {
				*cfg = ManagerConfig{}
			},
			wantErr: true,
			errContains: []string{
				"no markets provided",
				"exchange client cannot be nil",
				"signal caught up function cannot be nil",
				"job scheduler cannot be nil",
				"logger cannot be nil",
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
