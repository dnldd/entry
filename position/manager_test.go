package position

import (
	"context"
	"strings"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func setupManager(market string) (*Manager, chan string, *error) {
	notifyMsgs := make(chan string, 10)
	var persistClosedPositionErr error
	persistClosedPosition := func(pos *Position) error {
		return persistClosedPositionErr
	}

	cfg := &ManagerConfig{
		Markets: []string{market},
		Notify: func(message string) {
			notifyMsgs <- message
		},
		PersistClosedPosition: persistClosedPosition,
		Logger:                &log.Logger,
	}

	mgr := NewPositionManager(cfg)

	return mgr, notifyMsgs, &persistClosedPositionErr
}

func TestManager(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(market)

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
	}

	mgr.SendExitSignal(exitSignal)
	msg = <-notifyMsgs
	assert.True(t, strings.Contains(msg, "with stoploss"))

	marketStatusReq := shared.MarketStatusRequest{
		Market:   market,
		Response: make(chan shared.MarketStatus, 5),
	}

	mgr.SendMarketStatusRequest(marketStatusReq)
	<-marketStatusReq.Response

	// Ensure the position manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestFillManagerChannels(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr, _, _ := setupManager(market)

	// Ensure the position manager can process entry signals.
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
	}

	exitSignal := shared.ExitSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
	}

	marketStatusReq := shared.MarketStatusRequest{
		Market:   market,
		Response: make(chan shared.MarketStatus),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendEntrySignal(entrySignal)
		mgr.SendExitSignal(exitSignal)
		mgr.SendMarketStatusRequest(marketStatusReq)
	}

	assert.Equal(t, len(mgr.entrySignals), bufferSize)
	assert.Equal(t, len(mgr.exitSignals), bufferSize)
	assert.Equal(t, len(mgr.marketStatusRequests), bufferSize)
}

func TestHandleEntrySignals(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(market)

	// Ensure handling an entry signal for an unknown market errors.
	unknownMarketEntrySignal := shared.EntrySignal{
		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
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
	}

	err = mgr.handleEntrySignal(&entrySignal)
	assert.NoError(t, err)
	msg := <-notifyMsgs
	assert.True(t, strings.Contains(msg, "Created new long position"))
}

func TestHandleExitSignals(t *testing.T) {
	market := "^GSPC"
	mgr, notifyMsgs, _ := setupManager(market)

	// Create a valid position.
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
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
	}

	err = mgr.handleExitSignal(&exitSignal)
	assert.NoError(t, err)
	msg = <-notifyMsgs
	assert.True(t, strings.Contains(msg, "Closed long position"))
}

func TestHandleMarketStatusRequest(t *testing.T) {
	market := "^GSPC"
	mgr, _, _ := setupManager(market)

	// Ensure handling a request with an unknown market errors.
	unknownMarketStatusReq := shared.MarketStatusRequest{
		Market:   "^AAPL",
		Response: make(chan shared.MarketStatus),
	}

	err := mgr.handleMarketStatusRequest(&unknownMarketStatusReq)
	assert.Error(t, err)

	// Ensure a valid request is processed as expected.
	marketStatusReq := shared.MarketStatusRequest{
		Market:   market,
		Response: make(chan shared.MarketStatus, 5),
	}

	err = mgr.handleMarketStatusRequest(&marketStatusReq)
	assert.NoError(t, err)

	resp := <-marketStatusReq.Response
	assert.Equal(t, shared.NeutralInclination, resp)
}
