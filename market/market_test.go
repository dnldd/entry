package market

import (
	"fmt"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestMarket(t *testing.T) {
	// Ensure a market can be created.
	levelSignals := make(chan shared.LevelSignal, 2)
	signalLevel := func(signal *shared.LevelSignal) {
		levelSignals <- *signal
	}
	cfg := &MarketConfig{
		Market:      "^GSPC",
		SignalLevel: signalLevel,
	}

	mkt, err := NewMarket(cfg)
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

	// Ensure a market can be updated.
	candle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Date:   nextSessionTime,

		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
	}

	err = mkt.Update(candle)
	assert.NoError(t, err)
}
