package shared

import (
	"context"
	"testing"

	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

func TestHistoricalData(t *testing.T) {
	market := "^GSPC"
	caughtUpSignals := make(chan CaughtUpSignal, 1)
	signalCaughtUp := func(signal CaughtUpSignal) {
		caughtUpSignals <- signal
	}

	notifySubscribersSignals := make(chan Candlestick, 1)
	notifySubscribers := func(candle Candlestick) error {
		notifySubscribersSignals <- candle
		return nil
	}

	cfg := &HistoricDataConfig{
		Market:            market,
		FilePath:          "../testdata/historicdata.json",
		SignalCaughtUp:    signalCaughtUp,
		NotifySubscribers: notifySubscribers,
		Logger:            &log.Logger,
	}

	// Ensure historic data can be initialized.
	historicData, err := NewHistoricData(cfg)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	candleCount := atomic.NewInt32(0)
	caughUpCount := atomic.NewInt32(0)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case candle := <-notifySubscribersSignals:
				candle.Status <- Processed
				candleCount.Inc()
			case sig := <-caughtUpSignals:
				sig.Status <- Processed
				caughUpCount.Inc()
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		err = historicData.ProcessHistoricalData()
		assert.NoError(t, err)
		close(done)
	}()

	// Ensure the historical data process terminates gracefully.
	<-done
	cancel()

	// Ensure the start and end times of the historical data can be fetched.
	startTime := historicData.FetchStartTime()
	assert.Equal(t, startTime, historicData.candles[0].Date)
	endTime := historicData.FetchEndTime()
	assert.Equal(t, endTime, historicData.candles[len(historicData.candles)-1].Date)
	assert.Equal(t, candleCount.Load(), 11)
	assert.Equal(t, caughUpCount.Load(), 1)
}
