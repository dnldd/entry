package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestImbalanceSnapshot(t *testing.T) {
	// Ensure imbalance snapshot size cannot be negaitve or zero.
	timeframe := FiveMinute
	_, err := NewImbalanceSnapshot(-1, timeframe)
	assert.Error(t, err)

	imbalanceSnapshot, err := NewImbalanceSnapshot(0, timeframe)
	assert.Error(t, err)

	// Ensure an imbalance snapshot can be created.
	size := int32(4)
	imbalanceSnapshot, err = NewImbalanceSnapshot(size, timeframe)
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

	// Ensure calling last on an valid snapshot returns the last added entry.
	last = imbalanceSnapshot.Last()
	assert.NotNil(t, last)

	// Ensure calling LastN with a larger size than the snapshot gets clamped to the snapshot's size.
	lastN = imbalanceSnapshot.LastN(size + 1)
	assert.Equal(t, len(lastN), int(size))
}
