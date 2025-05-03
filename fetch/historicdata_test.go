package fetch

import (
	"context"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestHistoricalData(t *testing.T) {
	market := "^GSPC"
	caughtUpSignals := make(chan shared.CaughtUpSignal, 5)
	signalCaughtUp := func(signal shared.CaughtUpSignal) {
		caughtUpSignals <- signal
	}

	marketUpdateSignals := make(chan shared.Candlestick, 5)
	sendMarketUpdate := func(candle shared.Candlestick) {
		marketUpdateSignals <- candle
	}

	cfg := &HistoricDataConfig{
		Market:           market,
		Timeframe:        shared.FiveMinute,
		FilePath:         "../testdata/historicdata.json",
		SignalCaughtUp:   signalCaughtUp,
		SendMarketUpdate: sendMarketUpdate,
		Logger:           &log.Logger,
	}

	// Ensure historic data can be initialized.
	historicData, err := NewHistoricData(cfg)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	candleCount := 0
	caughUpCount := 0
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case candle := <-marketUpdateSignals:
				candle.Status <- shared.Processed
				candleCount++
			case sig := <-caughtUpSignals:
				sig.Status <- shared.Processed
				caughUpCount++
			}
		}
	}()

	go func() {
		err := historicData.ProcessHistoricalData()
		assert.NoError(t, err)
		cancel()
	}()

	// Ensure the historical data process terminates gracefully.
	<-done
	assert.Equal(t, candleCount, 8)
	assert.Equal(t, caughUpCount, 1)
}
