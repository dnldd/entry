package shared

import (
	"math"
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestVWAPSnapshot(t *testing.T) {
	// Ensure vwap snapshot size cannot be negaitve or zero.
	timeframe := FiveMinute
	vwapSnapshot, err := NewVWAPSnapshot(-1, timeframe)
	assert.Error(t, err)

	vwapSnapshot, err = NewVWAPSnapshot(0, timeframe)
	assert.Error(t, err)

	// Ensure a vwap snapshot can be created.
	size := int32(4)
	vwapSnapshot, err = NewVWAPSnapshot(size, timeframe)
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

	now, _, err := NewYorkTime()
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

	// Ensure vwap entries can be fetched by their associated timestamps.
	vwapAtTime := vwapSnapshot.At(now)
	assert.NotNil(t, vwapAtTime)
}

func TestVWAPTrend(t *testing.T) {
	tests := []struct {
		name  string
		data  []*VWAP
		trend Trend
		slope float64
		r2    float64
	}{
		{
			"no trend (chop)",
			[]*VWAP{
				{Value: 2}, {Value: 2}, {Value: 2}, {Value: 2}, {Value: 2},
				{Value: 2}, {Value: 2}, {Value: 2}, {Value: 2}, {Value: 2},
				{Value: 2}, {Value: 2}, {Value: 2}, {Value: 2}, {Value: 2},
				{Value: 2}, {Value: 2}, {Value: 2}, {Value: 2}, {Value: 2},
			},
			ChoppyTrend,
			0.0,
			0.0,
		},
		{
			"mild bullish trend",
			[]*VWAP{
				{Value: 2.00}, {Value: 2.04}, {Value: 1.98}, {Value: 2.06}, {Value: 2.00},
				{Value: 2.05}, {Value: 2.01}, {Value: 2.08}, {Value: 2.03}, {Value: 2.12},
				{Value: 2.05}, {Value: 2.10}, {Value: 2.07}, {Value: 2.12}, {Value: 2.08},
				{Value: 2.15}, {Value: 2.12}, {Value: 2.16}, {Value: 2.14}, {Value: 2.18},
			},
			MildBullishTrend,
			0.008556,
			0.760479,
		},
		{
			"strong linear bullish trend",
			[]*VWAP{
				{Value: 2}, {Value: 4}, {Value: 6}, {Value: 8}, {Value: 10},
				{Value: 12}, {Value: 14}, {Value: 16}, {Value: 18}, {Value: 20},
				{Value: 22}, {Value: 24}, {Value: 26}, {Value: 28}, {Value: 30},
				{Value: 32}, {Value: 34}, {Value: 36}, {Value: 38}, {Value: 40},
			},
			StrongBullishTrend,
			2.0,
			1.0,
		},
		{
			"strong parabolic bullish trend",
			[]*VWAP{
				{Value: 2}, {Value: 4}, {Value: 7}, {Value: 14}, {Value: 24},
				{Value: 36}, {Value: 54}, {Value: 67}, {Value: 84}, {Value: 102},
				{Value: 140}, {Value: 200}, {Value: 280}, {Value: 350}, {Value: 500},
				{Value: 700}, {Value: 1000}, {Value: 1500}, {Value: 2500}, {Value: 4000},
			},
			StrongBullishTrend,
			126.873684,
			0.539507,
		},
	}

	for _, test := range tests {
		size := int32(30)
		timeframe := FiveMinute
		snapshot, err := NewVWAPSnapshot(size, timeframe)
		if err != nil {
			t.Errorf("%s: unexpected error %v", test.name, err)
		}

		for idx := range test.data {
			vwap := test.data[idx]
			snapshot.Update(vwap)
		}

		trend, slope, r2 := snapshot.Trend(20)

		if math.Abs(slope-test.slope) > 0.0001 {
			t.Errorf("%s: mismatched slope, got %f, expected %f", test.name, slope, test.slope)
		}

		if math.Abs(r2-test.r2) > 0.0001 {
			t.Errorf("%s: mismatched r2, got %f, expected %f", test.name, r2, test.r2)
		}

		if trend != test.trend {
			t.Errorf("%s: mismatched trend , got %s, expected %s", test.name, trend.String(), test.trend.String())
		}
	}
}
