package service

import (
	"context"
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestEntryGracefulShutdown(t *testing.T) {
	market := "^GSPC"
	cfg := &EntryConfig{
		Markets:              []string{market},
		FMPAPIKey:            "key",
		Backtest:             true,
		BacktestMarket:       market,
		BacktestDataFilepath: "../testdata/historicdata.json",
	}

	entry, err := NewEntry(cfg)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure the entry service can be run and gracefully terminated.
	time.AfterFunc(time.Second*2, func() {
		cancel()
	})
	done := make(chan struct{})
	go func() {
		entry.Run(ctx)
		close(done)
	}()

	<-done
}
