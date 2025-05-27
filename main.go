package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/dnldd/entry/service"
)

// handleTermination processes context cancellation signals or interrupt signals from the OS.
func handleTermination(ctx context.Context, cancel context.CancelFunc) {
	// Listen for interrupt signals.
	signals := []os.Signal{os.Interrupt}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, signals...)

	// Wait for the context to be cancelled or an interrupt signal.
	for {
		select {
		case <-ctx.Done():
			return

		case <-interrupt:
			cancel()
		}
	}
}

func main() {
	var cfg Config
	err := loadConfig(&cfg, "")
	if err != nil {
		log.Printf("loading config:%v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entryCfg := service.EntryConfig{
		Markets:              cfg.Markets,
		FMPAPIKey:            cfg.FMPAPIKey,
		Backtest:             cfg.Backtest,
		BacktestDataFilepath: cfg.BacktestDataFilepath,
		Cancel:               cancel,
	}
	entry, err := service.NewEntry(&entryCfg)
	if err != nil {
		log.Printf("creating entry service: %v", err)
	}

	go handleTermination(ctx, cancel)
	entry.Run(ctx)
}
