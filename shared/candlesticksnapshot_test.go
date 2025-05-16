package shared

import (
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestCandlestickSnapshot(t *testing.T) {
	// Ensure candle snapshot size cannot be negaitve or zero.
	timeframe := FiveMinute
	candleSnapshot, err := NewCandlestickSnapshot(-1, timeframe)
	assert.Error(t, err)

	candleSnapshot, err = NewCandlestickSnapshot(0, timeframe)
	assert.Error(t, err)

	// Ensure a candlestick snapshot can be created.
	size := int32(4)
	candleSnapshot, err = NewCandlestickSnapshot(size, timeframe)
	assert.NoError(t, err)

	// Ensure calling last on an empty snapshot returns nothing.
	last := candleSnapshot.Last()
	assert.Nil(t, last)

	// Ensure calling LastN on an empty snapshot returns an empty set.
	lastN := candleSnapshot.LastN(size)
	assert.Equal(t, len(lastN), 0)

	// Ensure calling LastN with zero or negagive size returns nil.
	lastN = candleSnapshot.LastN(-1)
	assert.Nil(t, lastN)

	// Ensure the snapshot can be updated with candles.
	for idx := range size {
		candle := &Candlestick{
			Open:      float64(idx + 1),
			Close:     float64(idx + 2),
			High:      float64(idx + 3),
			Low:       float64(idx),
			Volume:    float64(idx),
			Status:    make(chan StatusCode, 1),
			Timeframe: timeframe,
		}
		err = candleSnapshot.Update(candle)
		assert.NoError(t, err)
	}

	assert.Equal(t, candleSnapshot.count.Load(), size)
	assert.Equal(t, candleSnapshot.size.Load(), size)
	assert.Equal(t, candleSnapshot.start.Load(), 0)
	assert.Equal(t, len(candleSnapshot.data), int(size))

	// Ensure calling last on an valid snapshot returns the last added entry.
	last = candleSnapshot.Last()
	assert.Equal(t, last.Low, float64(3))

	// Ensure calling LastN with a larger size than the snapshot gets clamped to the snapshot's size.
	lastN = candleSnapshot.LastN(size + 1)
	assert.Equal(t, len(lastN), int(size))

	// Ensure candle updates at capacity overwrite existing slots.
	candle := &Candlestick{
		Open:      float64(5),
		Close:     float64(8),
		High:      float64(9),
		Low:       float64(3),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	err = candleSnapshot.Update(candle)
	assert.NoError(t, err)
	assert.Equal(t, candleSnapshot.count.Load(), size)
	assert.Equal(t, candleSnapshot.size.Load(), size)
	assert.Equal(t, candleSnapshot.start.Load(), 1)
	assert.Equal(t, len(candleSnapshot.data), int(size))

	// Ensure the last n elements can be fetched from the snapshot.
	nSet := candleSnapshot.LastN(2)
	expectedCandle := Candlestick{
		Open:   4,
		Close:  5,
		High:   6,
		Low:    3,
		Volume: 3,
	}
	assert.Equal(t, nSet[0].Open, expectedCandle.Open)
	assert.Equal(t, nSet[0].High, expectedCandle.High)
	assert.Equal(t, nSet[0].Low, expectedCandle.Low)
	assert.Equal(t, nSet[0].Close, expectedCandle.Close)
	assert.Equal(t, nSet[0].Volume, expectedCandle.Volume)
	assert.Equal(t, nSet[1].Open, candle.Open)
	assert.Equal(t, nSet[1].High, candle.High)
	assert.Equal(t, nSet[1].Low, candle.Low)
	assert.Equal(t, nSet[1].Close, candle.Close)
	assert.Equal(t, nSet[1].Volume, candle.Volume)

	// Ensure the average volume n can be fetched from the snapshot.
	average := candleSnapshot.AverageVolumeN(2)
	assert.Equal(t, average, 2.5)

	// Ensure calling average volume clamps n to the size of the snapshot if it exceeds it.
	average = candleSnapshot.AverageVolumeN(6)
	assert.Equal(t, average, 2)

	// Ensure candle updates after capacity advances the start index for the next addition.
	next := &Candlestick{
		Open:      float64(6),
		Close:     float64(9),
		High:      float64(10),
		Low:       float64(4),
		Volume:    float64(3),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	err = candleSnapshot.Update(next)
	assert.NoError(t, err)
	assert.Equal(t, candleSnapshot.count.Load(), size)
	assert.Equal(t, candleSnapshot.size.Load(), size)
	assert.Equal(t, candleSnapshot.start.Load(), 2)
	assert.Equal(t, len(candleSnapshot.data), int(size))

	// Ensure updating the snapshot with a candle of a different timeframe errors.
	wrongTimeframeCandle := &Candlestick{
		Open:      float64(6),
		Close:     float64(9),
		High:      float64(10),
		Low:       float64(4),
		Volume:    float64(3),
		Status:    make(chan StatusCode, 1),
		Timeframe: OneHour,
	}

	err = candleSnapshot.Update(wrongTimeframeCandle)
	assert.Error(t, err)
}
