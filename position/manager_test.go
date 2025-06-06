package position

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

func TestPositionManagerConfigValidate(t *testing.T) {
	// Dummy functions for required fields
	dummyNotify := func(message string) {}
	dummyPersistClosedPosition := func(position *Position) error { return nil }

	// Use a real zerolog.Logger and gocron.Scheduler for testing
	logger := zerolog.New(nil)
	scheduler := gocron.NewScheduler(time.UTC)

	baseCfg := &ManagerConfig{
		Markets:               []string{"AAPL"},
		Notify:                dummyNotify,
		Backtest:              false,
		PersistClosedPosition: dummyPersistClosedPosition,
		JobScheduler:          scheduler,
		Logger:                &logger,
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
			name:        "missing Notify",
			modify:      func(cfg *ManagerConfig) { cfg.Notify = nil },
			wantErr:     true,
			errContains: []string{"notify function cannot be nil"},
		},
		{
			name:        "missing PersistClosedPosition",
			modify:      func(cfg *ManagerConfig) { cfg.PersistClosedPosition = nil },
			wantErr:     true,
			errContains: []string{"persist closed position function cannot be nil"},
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
				"notify function cannot be nil",
				"persist closed position function cannot be nil",
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

func setupManager(t *testing.T, market string) (*Manager, chan string, *error) {
	notifyMsgs := make(chan string, 10)
	var persistClosedPositionErr error
	persistClosedPosition := func(pos *Position) error {
		return persistClosedPositionErr
	}

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &ManagerConfig{
		Markets: []string{market},
		Notify: func(message string) {
			notifyMsgs <- message
		},
		PersistClosedPosition: persistClosedPosition,
		JobScheduler:          gocron.NewScheduler(loc),
		Logger:                &log.Logger,
	}

	mgr, err := NewPositionManager(cfg)
	assert.NoError(t, err)

	return mgr, notifyMsgs, &persistClosedPositionErr
}

func TestManager(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(t, market)

	// Ensure the position manager can be started.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure the position manager can process entry signals.
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Status:    make(chan shared.StatusCode, 1),
	}

	mgr.SendEntrySignal(entrySignal)
	msg := <-notifyMsgs
	assert.True(t, strings.Contains(msg, "with stoploss"))

	// Ensure the position manager can process exit signals.
	exitSignal := shared.ExitSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		Status:    make(chan shared.StatusCode, 1),
	}

	mgr.SendExitSignal(exitSignal)
	msg = <-notifyMsgs
	assert.True(t, strings.Contains(msg, "with stoploss"))

	marketSkewReq := shared.MarketSkewRequest{
		Market:   market,
		Response: make(chan shared.MarketSkew, 5),
	}

	mgr.SendMarketSkewRequest(marketSkewReq)
	<-marketSkewReq.Response

	// Ensure the position manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr, _, _ := setupManager(t, market)

	// Ensure the position manager can process entry signals.
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Status:    make(chan shared.StatusCode, 1),
	}

	exitSignal := shared.ExitSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		Status:    make(chan shared.StatusCode, 1),
	}

	marketSkewReq := shared.MarketSkewRequest{
		Market:   market,
		Response: make(chan shared.MarketSkew),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendEntrySignal(entrySignal)
		mgr.SendExitSignal(exitSignal)
		mgr.SendMarketSkewRequest(marketSkewReq)
	}

	assert.Equal(t, len(mgr.entrySignals), bufferSize)
	assert.Equal(t, len(mgr.exitSignals), bufferSize)
	assert.Equal(t, len(mgr.marketSkewRequests), bufferSize)
}

func TestHandleEntrySignals(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(t, market)

	// Ensure handling an entry signal for an unknown market errors.
	unknownMarketEntrySignal := shared.EntrySignal{
		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleEntrySignal(&unknownMarketEntrySignal)
	assert.Error(t, err)

	// Ensure a valid entry signal gets processed as expected.
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleEntrySignal(&entrySignal)
	assert.NoError(t, err)
	msg := <-notifyMsgs
	assert.True(t, strings.Contains(msg, "Created new long position"))
}

func TestHandleExitSignals(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(t, market)

	// Create a valid
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleEntrySignal(&entrySignal)
	assert.NoError(t, err)
	msg := <-notifyMsgs
	assert.True(t, strings.Contains(msg, "Created new long position"))

	// Ensure an exit signal with an unknown market errors.
	unknownMarketExitSignal := shared.ExitSignal{
		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleExitSignal(&unknownMarketExitSignal)
	assert.Error(t, err)

	// Ensure a valid exit signal gets processed as expected.
	exitSignal := shared.ExitSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleExitSignal(&exitSignal)
	assert.NoError(t, err)
	msg = <-notifyMsgs
	assert.True(t, strings.Contains(msg, "Closed long position"))
}

func TestHandleMarketStatusRequest(t *testing.T) {
	market := "^GSPC"
	mgr, _, _ := setupManager(t, market)

	// Ensure handling a request with an unknown market errors.
	unknownMarketSkewReq := shared.MarketSkewRequest{
		Market:   "^AAPL",
		Response: make(chan shared.MarketSkew),
	}

	err := mgr.handleMarketSkewRequest(&unknownMarketSkewReq)
	assert.Error(t, err)

	// Ensure a valid request is processed as expected.
	skewReq := shared.MarketSkewRequest{
		Market:   market,
		Response: make(chan shared.MarketSkew, 5),
	}

	err = mgr.handleMarketSkewRequest(&skewReq)
	assert.NoError(t, err)

	resp := <-skewReq.Response
	assert.Equal(t, shared.NeutralSkew, resp)
}
