package shared

import (
	"context"
	"testing"

	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestHistoricalData(t *testing.T) {
	market := "^GSPC"
	caughtUpSignals := make(chan CaughtUpSignal, 5)
	signalCaughtUp := func(signal CaughtUpSignal) {
		caughtUpSignals <- signal
	}

	marketUpdateSignals := make(chan Candlestick, 5)
	sendMarketUpdate := func(candle Candlestick) {
		marketUpdateSignals <- candle
	}

	cfg := &HistoricDataConfig{
		Market:           market,
		Timeframe:        FiveMinute,
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
				candle.Status <- Processed
				candleCount++
			case sig := <-caughtUpSignals:
				sig.Status <- Processed
				caughUpCount++
			}
		}
	}()

	go func() {
		err := historicData.ProcessHistoricalData()
		assert.NoError(t, err)
		cancel()
	}()

	// Ensure the start and end times of the historical data can be fetched.
	startTime := historicData.FetchStartTime()
	assert.Equal(t, startTime, historicData.candles[0].Date)
	endTime := historicData.FetchEndTime()
	assert.Equal(t, endTime, historicData.candles[len(historicData.candles)-1].Date)

	// Ensure the historical data process terminates gracefully.
	<-done
	assert.Equal(t, candleCount, 8)
	assert.Equal(t, caughUpCount, 1)
}
