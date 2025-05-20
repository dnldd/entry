package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestImbalanceUpdate(t *testing.T) {
	// Ensure an imbalance can be created.
	market := "^GSPC"
	timeframe := FiveMinute
	high := float64(23)
	midpoint := float64(20.5)
	low := float64(18)
	gapRatio := float64(0.7142857142857143)
	bullishImbalance := NewImbalance(market, timeframe, high, midpoint, low, Bullish, gapRatio, time.Time{})
	bearishImbalance := NewImbalance(market, timeframe, low, midpoint, high, Bearish, gapRatio, time.Time{})

	// Ensure an imbalance can be updated by new candlestick data.
	bullishPurgeCandle := &Candlestick{
		Market:    market,
		Open:      float64(25),
		Close:     float64(16),
		High:      float64(26),
		Low:       float64(14),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	// Ensure an imbalance can be updated by new candlestick data.
	bearishPurgeCandle := &Candlestick{
		Market:    market,
		Open:      float64(14),
		Close:     float64(30),
		High:      float64(32),
		Low:       float64(13),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	bullishImbalance.Update(bullishPurgeCandle)
	assert.True(t, bullishImbalance.Purged.Load())

	bearishImbalance.Update(bearishPurgeCandle)
	assert.True(t, bullishImbalance.Purged.Load())

	// Ensure a subsequente close beyond the imbalance invalidates it.
	bullishImbalance.Update(bullishPurgeCandle)
	assert.True(t, bullishImbalance.Invalidated.Load())

	bearishImbalance.Update(bearishPurgeCandle)
	assert.True(t, bullishImbalance.Invalidated.Load())
}
