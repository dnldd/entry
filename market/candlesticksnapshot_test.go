package market

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestCandlestickSnapshot(t *testing.T) {
	// Ensure candle snapshot size cannot be negaitve or zero.
	candleSnapshot, err := NewCandlestickSnapshot(-1)
	assert.Error(t, err)

	candleSnapshot, err = NewCandlestickSnapshot(0)
	assert.Error(t, err)

	// Ensure a candlestick snapshot can be created.
	size := 4
	candleSnapshot, err = NewCandlestickSnapshot(size)
	assert.NoError(t, err)

	// Ensure the snapshot can be updated with candles.
	for idx := range size {
		candle := &shared.Candlestick{
			Open:   float64(idx + 1),
			Close:  float64(idx + 2),
			High:   float64(idx + 3),
			Low:    float64(idx),
			Volume: float64(idx),
		}
		candleSnapshot.Update(candle)
	}

	assert.Equal(t, candleSnapshot.count, size)
	assert.Equal(t, candleSnapshot.size, size)
	assert.Equal(t, candleSnapshot.start, 0)
	assert.Equal(t, len(candleSnapshot.data), size)

	// Ensure candle updates at capacity overwrite existing slots.
	candle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
	}

	candleSnapshot.Update(candle)
	assert.Equal(t, candleSnapshot.count, size)
	assert.Equal(t, candleSnapshot.size, size)
	assert.Equal(t, candleSnapshot.start, 1)
	assert.Equal(t, len(candleSnapshot.data), size)

	// Ensure the last n elements can be fetched from the snapshot.
	nSet := candleSnapshot.LastN(2)
	assert.Equal(t, nSet[0],
		&shared.Candlestick{
			Open:   4,
			Close:  5,
			High:   6,
			Low:    3,
			Volume: 3,
		})
	assert.Equal(t, nSet[1], candle)

	// Ensure the average volume n can be fetched from the snapshot.
	average := candleSnapshot.AverageVolumeN(2)
	assert.Equal(t, average, 2.5)

	// Ensure candle updates after capacity advances the start index for the next addition.
	next := &shared.Candlestick{
		Open:   float64(6),
		Close:  float64(9),
		High:   float64(10),
		Low:    float64(4),
		Volume: float64(3),
	}

	candleSnapshot.Update(next)
	assert.Equal(t, candleSnapshot.count, size)
	assert.Equal(t, candleSnapshot.size, size)
	assert.Equal(t, candleSnapshot.start, 2)
	assert.Equal(t, len(candleSnapshot.data), size)

}
