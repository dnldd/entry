package indicator

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestVWAPSnapshot(t *testing.T) {
	// Ensure vwap snapshot size cannot be negaitve or zero.
	vwapSnapshot, err := NewVWAPSnapshot(-1)
	assert.Error(t, err)

	vwapSnapshot, err = NewVWAPSnapshot(0)
	assert.Error(t, err)

	// Ensure a vwap snapshot can be created.
	size := int32(4)
	vwapSnapshot, err = NewVWAPSnapshot(size)
	assert.NoError(t, err)

	// Ensure calling last on an empty snapshot returns nothing.
	last := vwapSnapshot.Last()
	assert.Nil(t, last)

	// Ensure calling LastN on an empty snapshot returns an empty set.
	lastN := vwapSnapshot.LastN(size)
	assert.Equal(t, len(lastN), 0)

	// Ensure calling LastN with zero or negagive size returns nil.
	lastN = vwapSnapshot.LastN(-1)
	assert.Nil(t, lastN)

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure the snapshot can be updated with vwap.
	for idx := range size {
		t := now.AddDate(0, 0, int(idx))
		vwap := &VWAP{
			Value: float64(idx + 1),
			Date:  t,
		}

		vwapSnapshot.Update(vwap)
	}

	assert.Equal(t, vwapSnapshot.count.Load(), size)
	assert.Equal(t, vwapSnapshot.size.Load(), size)
	assert.Equal(t, vwapSnapshot.start.Load(), 0)
	assert.Equal(t, len(vwapSnapshot.data), int(size))

	// Ensure calling last on an valid snapshot returns the last added entry.
	last = vwapSnapshot.Last()
	assert.Equal(t, last.Value, float64(4))

	// Ensure calling LastN with a larger size than the snapshot gets clamped to the snapshot's size.
	lastN = vwapSnapshot.LastN(size + 1)
	assert.Equal(t, len(lastN), int(size))

	// Ensure vwap updates at capacity overwrite existing slots.
	vwap := &VWAP{
		Value: float64(5),
		Date:  now,
	}

	vwapSnapshot.Update(vwap)
	assert.Equal(t, vwapSnapshot.count.Load(), size)
	assert.Equal(t, vwapSnapshot.size.Load(), size)
	assert.Equal(t, vwapSnapshot.start.Load(), 1)
	assert.Equal(t, len(vwapSnapshot.data), int(size))

	// Ensure the last n elements can be fetched from the snapshot.
	nSet := vwapSnapshot.LastN(2)
	lastButOneVwap := VWAP{
		Value: 4,
	}
	assert.Equal(t, nSet[0].Value, lastButOneVwap.Value)
	assert.Equal(t, nSet[1].Value, vwap.Value)

	// Ensure vwap updates after capacity advances the start index for the next addition.
	next := &VWAP{
		Value: 6,
	}

	vwapSnapshot.Update(next)
	assert.Equal(t, vwapSnapshot.count.Load(), size)
	assert.Equal(t, vwapSnapshot.size.Load(), size)
	assert.Equal(t, vwapSnapshot.start.Load(), 2)
	assert.Equal(t, len(vwapSnapshot.data), int(size))

	// Ensure vwap entries can be fetched by their associated date times.
	vwapAtTime := vwapSnapshot.At(now)
	assert.NotNil(t, vwapAtTime)
}
