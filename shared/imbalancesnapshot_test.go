package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestImbalanceSnapshot(t *testing.T) {
	// Ensure imbalance snapshot size cannot be negaitve or zero.
	_, err := NewImbalanceSnapshot(-1)
	assert.Error(t, err)

	imbalanceSnapshot, err := NewImbalanceSnapshot(0)
	assert.Error(t, err)

	// Ensure an imbalance snapshot can be created.
	size := int32(4)
	imbalanceSnapshot, err = NewImbalanceSnapshot(size)
	assert.NoError(t, err)

	// Ensure calling last on an empty snapshot returns nothing.
	last := imbalanceSnapshot.Last()
	assert.Nil(t, last)

	// Ensure calling LastN on an empty snapshot returns an empty set.
	lastN := imbalanceSnapshot.LastN(size)
	assert.Equal(t, len(lastN), 0)

	// Ensure calling LastN with zero or negative size returns nil.
	lastN = imbalanceSnapshot.LastN(-1)
	assert.Nil(t, lastN)

	market := "^GSPC"
	high := float64(23)
	midpoint := float64(20.5)
	timeframe := FiveMinute
	low := float64(18)
	gapRatio := float64(0.7142857142857143)
	imbalance := NewImbalance(market, timeframe, high, midpoint, low, Bullish, gapRatio, time.Time{})

	// Ensure the snapshot can be updated with candles.
	for range size {
		err := imbalanceSnapshot.Add(imbalance)
		assert.NoError(t, err)
	}

	assert.Equal(t, imbalanceSnapshot.count.Load(), size)
	assert.Equal(t, imbalanceSnapshot.size.Load(), size)
	assert.Equal(t, imbalanceSnapshot.start.Load(), 0)
	assert.Equal(t, len(imbalanceSnapshot.data), int(size))

	// Ensure the snapshot overwrites its store at capacity.
	err = imbalanceSnapshot.Add(imbalance)
	assert.NoError(t, err)

	assert.Equal(t, imbalanceSnapshot.start.Load(), 1)

	// Ensure calling last on an valid snapshot returns the last added entry.
	last = imbalanceSnapshot.Last()
	assert.NotNil(t, last)

	// Ensure calling LastN with a larger size than the snapshot gets clamped to the snapshot's size.
	lastN = imbalanceSnapshot.LastN(size + 1)
	assert.Equal(t, len(lastN), int(size))

	// Ensure the snapshot can  process market updates.
	candle := &Candlestick{
		Market:    market,
		Open:      float64(25),
		Close:     float64(16),
		High:      float64(26),
		Low:       float64(14),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	imbalanceSnapshot.Update(candle)
	assert.True(t, imbalanceSnapshot.Last().Purged.Load())

	// Ensure the snapshot can be filtered for qualifying imbalances.
	filterFunc := func(*Imbalance, *Candlestick) bool {
		if imbalance.Low > candle.Close {
			return true
		}

		return false
	}

	imbalanceSet := imbalanceSnapshot.Filter(candle, filterFunc)
	assert.GreaterThan(t, len(imbalanceSet), 0)
}
