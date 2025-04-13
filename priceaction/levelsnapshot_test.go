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
	size := 4
	levelSnapshot, err = NewLevelSnapshot(size)
	assert.NoError(t, err)

	// Ensure the snapshot can be updated with levels.
	price := float64(12)
	market := "^GSPC"
	resistanceCandle := &shared.Candlestick{
		Open:  10,
		High:  15,
		Low:   9,
		Close: 5,
	}
	supportCandle := &shared.Candlestick{
		Open:  13,
		High:  18,
		Low:   12,
		Close: 17,
	}
	for idx := range size {
		var level *shared.Level
		if idx%2 == 0 {
			level = shared.NewLevel(market, price, resistanceCandle)
		} else {
			level = shared.NewLevel(market, price, supportCandle)
		}

		levelSnapshot.Add(level)
	}

	assert.Equal(t, levelSnapshot.count, size)
	assert.Equal(t, levelSnapshot.size, size)
	assert.Equal(t, levelSnapshot.start, 0)
	assert.Equal(t, len(levelSnapshot.data), size)

	// Ensure level updates at capacity overwrite existing slots.
	level := shared.NewLevel(market, price, resistanceCandle)
	levelSnapshot.Add(level)

	assert.Equal(t, levelSnapshot.count, size)
	assert.Equal(t, levelSnapshot.size, size)
	assert.Equal(t, levelSnapshot.start, 1)
	assert.Equal(t, len(levelSnapshot.data), size)

	// Ensure the snapshot can be filtered.
	filter := func(level *shared.Level, candle *shared.Candlestick) bool {
		return level.Price > candle.Close

	}

	filteredLevels := levelSnapshot.Filter(resistanceCandle, filter)
	assert.GreaterThan(t, len(filteredLevels), 0)
}
