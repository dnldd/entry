package priceaction

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestLevelSnapshot(t *testing.T) {
	// Ensure level snapshot size cannot be negaitve or zero.
	levelSnapshot, err := NewLevelSnapshot(-1)
	assert.Error(t, err)

	levelSnapshot, err = NewLevelSnapshot(0)
	assert.Error(t, err)

	// Ensure a candlestick snapshot can be created.
	size := int32(4)
	levelSnapshot, err = NewLevelSnapshot(size)
	assert.NoError(t, err)

	// Ensure the snapshot can be updated with levels.
	price := float64(12)
	market := "^GSPC"
	resistanceCandle := &shared.Candlestick{
		Open:   10,
		High:   15,
		Low:    9,
		Close:  5,
		Status: make(chan shared.StatusCode, 1),
	}

	supportClose := float64(17)

	for idx := range size {
		var level *shared.Level
		if idx%2 == 0 {
			level = shared.NewLevel(market, price, resistanceCandle.Close)
		} else {
			level = shared.NewLevel(market, price, supportClose)
		}

		levelSnapshot.Add(level)
	}

	assert.Equal(t, levelSnapshot.count.Load(), size)
	assert.Equal(t, levelSnapshot.size.Load(), size)
	assert.Equal(t, levelSnapshot.start.Load(), 0)
	assert.Equal(t, len(levelSnapshot.data), int(size))

	// Ensure level updates at capacity overwrite existing slots.
	level := shared.NewLevel(market, price, resistanceCandle.Close)
	levelSnapshot.Add(level)

	assert.Equal(t, levelSnapshot.count.Load(), size)
	assert.Equal(t, levelSnapshot.size.Load(), size)
	assert.Equal(t, levelSnapshot.start.Load(), 1)
	assert.Equal(t, len(levelSnapshot.data), int(size))

	// Ensure the snapshot can be filtered.
	filter := func(level *shared.Level, candle *shared.Candlestick) bool {
		return level.Price > candle.Close
	}

	filteredLevels := levelSnapshot.Filter(resistanceCandle, filter)
	assert.GreaterThan(t, len(filteredLevels), 0)
}
