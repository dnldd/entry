package market

import (
	"fmt"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestMarket(t *testing.T) {
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure a market can be created.
	levelSignals := make(chan shared.LevelSignal, 2)
	signalLevel := func(signal *shared.LevelSignal) {
		levelSignals <- *signal
	}
	cfg := &MarketConfig{
		Market:      "^GSPC",
		SignalLevel: signalLevel,
		Logger:      &log.Logger,
	}

	mkt, err := NewMarket(cfg, now)
	assert.NoError(t, err)

	// Ensure a market's caught status can be set and fetched.
	mkt.SetCaughtUpStatus(true)
	status := mkt.CaughtUp()
	assert.Equal(t, status, true)

	currentSession := mkt.sessionSnapshot.FetchCurrentSession()

	// Set the update candle's time to a time in the next session
	// to trigger level signals.
	fmt.Println(currentSession.Name)
	fmt.Println(currentSession.Close)
	nextSessionTime := currentSession.Close.Add(time.Hour * 2)
	fmt.Println(nextSessionTime)

	// Ensure a market ignores candle updates that are not of the expected update timeframe (five minute timeframe).
	ignoredCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    "^GSPC",
		Timeframe: shared.OneHour,
	}

	err = mkt.Update(ignoredCandle)
	assert.NoError(t, err)

	// Ensure a market can be updated.
	firstCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   now,

		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
	}

	err = mkt.Update(firstCandle)
	assert.NoError(t, err)

	// Ensure a market can trigger session high/low signals.
	secondCandle := &shared.Candlestick{
		Open:   float64(9),
		Close:  float64(12),
		High:   float64(15),
		Low:    float64(8),
		Volume: float64(3),
		Date:   nextSessionTime,

		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
	}

	err = mkt.Update(secondCandle)
	assert.NoError(t, err)

	levelHigh := <-levelSignals
	levelLow := <-levelSignals

	assert.Equal(t, levelHigh.Price, float64(9))
	assert.Equal(t, levelLow.Price, float64(3))
}
