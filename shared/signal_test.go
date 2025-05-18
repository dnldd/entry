package shared

import (
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestSignalStatus(t *testing.T) {
	// Ensure signals can be created and can receive status updates on their corresponding channels.
	market := "^GSPC"
	timeframe := FiveMinute
	now, _, _ := NewYorkTime()

	entrySignal := NewEntrySignal(market, timeframe, Long, float64(10),
		[]Reason{BullishEngulfing, StrongMove, StrongVolume}, 8, now, 6, float64(2))
	assert.NotNil(t, entrySignal)
	go func() { entrySignal.Status <- Processed }()
	status := <-entrySignal.Status
	assert.Equal(t, status, Processed)

	exitSignal := NewExitSignal(market, timeframe, Long, float64(20), []Reason{BearishEngulfing, StrongMove}, 8, now)
	assert.NotNil(t, entrySignal)
	go func() { exitSignal.Status <- Processed }()
	status = <-exitSignal.Status
	assert.Equal(t, status, Processed)

	levelSignal := NewLevelSignal(market, float64(14), float64(16))
	assert.NotNil(t, levelSignal)
	go func() { levelSignal.Status <- Processed }()
	status = <-levelSignal.Status
	assert.Equal(t, status, Processed)

	catchUpSignal := NewCatchUpSignal(market, timeframe, now)
	assert.NotNil(t, catchUpSignal)
	go func() { catchUpSignal.Status <- Processed }()
	status = <-catchUpSignal.Status
	assert.Equal(t, status, Processed)

	caughtUpSignal := NewCaughtUpSignal(market)
	assert.NotNil(t, caughtUpSignal)
	go func() { caughtUpSignal.Status <- Processed }()
	status = <-caughtUpSignal.Status
	assert.Equal(t, status, Processed)
}
