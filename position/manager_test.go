package position

import (
	"context"
	"strings"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestManager(t *testing.T) {
	// Ensure a position manager can be created.
	notifyMsgs := make(chan string, 5)
	var persistClosedPositionErr error
	persistClosedPosition := func(pos *Position) error {
		return persistClosedPositionErr
	}
	cfg := &ManagerConfig{
		Notify: func(message string) {
			notifyMsgs <- message
		},
		PersistClosedPosition: persistClosedPosition,
		Logger:                &log.Logger,
	}

	mgr := NewPositionManager(cfg)

	// Ensure the position manager can be started.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure the position manager can process entry signals.
	market := "^GSPC"
	entrySignal := shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(10),
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  float64(8),
		Done:      make(chan struct{}),
	}

	mgr.SendEntrySignal(entrySignal)
	<-entrySignal.Done
	msg := <-notifyMsgs
	assert.True(t, strings.Contains(msg, "with stoploss"))
	mgr.positionsMtx.RLock()
	assert.Equal(t, len(mgr.positions), 1)
	mgr.positionsMtx.RUnlock()

	// Ensure the position manager can process exit signals.
	exitSignal := shared.ExitSignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     float64(15),
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		Done:      make(chan struct{}),
	}

	mgr.SendExitSignal(exitSignal)
	msg = <-notifyMsgs
	assert.True(t, strings.Contains(msg, "with stoploss"))
	mgr.positionsMtx.RLock()
	assert.Equal(t, len(mgr.positions), 0)
	mgr.positionsMtx.RUnlock()

	// Ensure the position manager can be gracefully shutdown.
	cancel()
	<-done
}
