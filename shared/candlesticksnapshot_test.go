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

	// Ensure calling LastN with zero or negative size returns nil.
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
		Timeframe: Timeframe(999),
	}

	err = candleSnapshot.Update(wrongTimeframeCandle)
	assert.Error(t, err)
}

func TestDetectImbalance(t *testing.T) {
	size := int32(8)
	timeframe := FiveMinute
	market := "^GSPC"

	tests := []struct {
		name          string
		candles       []Candlestick
		wantImbalance bool
		sentiment     Sentiment
		gapRatio      float64
		high          float64
		low           float64
	}{
		{
			"no imbalance - no candle gaps",
			[]Candlestick{
				{
					Market:    market,
					Open:      float64(5),
					Close:     float64(10),
					High:      float64(12),
					Low:       float64(4),
					Volume:    float64(3),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(10),
					Close:     float64(15),
					High:      float64(16),
					Low:       float64(5),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(15),
					Close:     float64(17),
					High:      float64(18),
					Low:       float64(10),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
			},
			false,
			Sentiment(999),
			0.0,
			0.0,
			0.0,
		},
		{
			"no imbalance - low volume",
			[]Candlestick{
				{
					Market:    market,
					Open:      float64(15),
					Close:     float64(17),
					High:      float64(18),
					Low:       float64(10),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(17),
					Close:     float64(24),
					High:      float64(25),
					Low:       float64(16),
					Volume:    float64(1),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(24),
					Close:     float64(27),
					High:      float64(28),
					Low:       float64(23),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
			},
			false,
			Sentiment(999),
			0.0,
			0.0,
			0.0,
		},
		{
			"bullish imbalance",
			[]Candlestick{
				{
					Market:    market,
					Open:      float64(15),
					Close:     float64(17),
					High:      float64(18),
					Low:       float64(10),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(17),
					Close:     float64(24),
					High:      float64(25),
					Low:       float64(16),
					Volume:    float64(7),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(24),
					Close:     float64(27),
					High:      float64(28),
					Low:       float64(23),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
			},
			true,
			Bullish,
			0.7142857142857143,
			23.0,
			18.0,
		},
		{
			"bearish imbalance",
			[]Candlestick{
				{
					Market:    market,
					Open:      float64(15),
					Close:     float64(14),
					High:      float64(16),
					Low:       float64(13),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(14),
					Close:     float64(8),
					High:      float64(15),
					Low:       float64(7),
					Volume:    float64(7),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
				{
					Market:    market,
					Open:      float64(8),
					Close:     float64(7),
					High:      float64(9),
					Low:       float64(6),
					Volume:    float64(2),
					Status:    make(chan StatusCode, 1),
					Timeframe: timeframe,
				},
			},
			true,
			Bearish,
			0.6666666666666666,
			13.0,
			9.0,
		},
	}

	for _, test := range tests {
		snapshot, err := NewCandlestickSnapshot(size, timeframe)
		assert.NoError(t, err)

		for idx := range test.candles {
			candle := test.candles[idx]
			snapshot.Update(&candle)
		}

		imbalance, ok := snapshot.DetectImbalance()

		if (!test.wantImbalance && ok) || (test.wantImbalance && !ok) {
			t.Errorf("%s: expected %v, got %v", test.name, test.wantImbalance, ok)
		}

		if test.wantImbalance && ok {
			if test.gapRatio != imbalance.GapRatio {
				t.Errorf("%s: expected imbalance gap ratio %.2f, got %.2f", test.name, imbalance.GapRatio, test.gapRatio)
			}

			if test.sentiment != imbalance.Sentiment {
				t.Errorf("%s: expected imbalance sentiment %s, got %s", test.name, imbalance.Sentiment, test.sentiment)
			}

			if test.high != imbalance.High {
				t.Errorf("%s: expected imbalance high %.2f, got %.2f", test.name, imbalance.High, test.high)
			}

			if test.low != imbalance.Low {
				t.Errorf("%s: expected imbalance low %.2f, got %.2f", test.name, imbalance.Low, test.low)
			}
		}
	}
}
