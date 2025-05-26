package service

import (
	"context"
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestEntryRun(t *testing.T) {
	// Ensure the entry service can be created and run.
	market := "^GSPC"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := EntryConfig{
		Markets:   []string{market},
		FMPAPIKey: "key",
		Backtest:  false,
		Cancel:    cancel,
	}
	entry, err := NewEntry(&cfg)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		entry.Run(ctx)
		close(done)
	}()

	cancel()
	<-done
}

func TestEntryBacktest(t *testing.T) {
	// Ensure the entry service can run a backtest.
	market := "^GSPC"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := EntryConfig{
		Markets:              []string{market},
		FMPAPIKey:            "key",
		Backtest:             true,
		BacktestMarket:       market,
		BacktestDataFilepath: "../testdata/historicdata.json",
		Cancel:               cancel,
	}
	entry, err := NewEntry(&cfg)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		entry.Run(ctx)
		close(done)
	}()

	<-done
}
