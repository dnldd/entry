package priceaction

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestMarket(t *testing.T) {
	market := "^GSPC"

	// Ensure a market can be created.
	mkt, err := NewMarket(market)
	assert.NoError(t, err)
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(0))
	assert.Equal(t, mkt.requestingPriceData.Load(), false)
	// Ensure a market can be updated with price data.
	supportCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
	}

	resistanceCandle := &shared.Candlestick{
		Open:   float64(8),
		Close:  float64(9),
		High:   float64(10),
		Low:    float64(5),
		Volume: float64(2),
	}

	mkt.Update(supportCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(0))
	assert.Equal(t, mkt.requestingPriceData.Load(), false)

	// Ensure levels can be added to the market.
	supportPrice := float64(2)
	level := shared.NewLevel(market, supportPrice, supportCandle)
	mkt.AddLevel(level)

	invalidLevel := shared.NewLevel(market, supportPrice, supportCandle)
	invalidLevel.Invalidated.Store(true)
	mkt.AddLevel(invalidLevel)

	resistancePrice := float64(10)
	secondLevel := shared.NewLevel(market, resistancePrice, resistanceCandle)
	mkt.AddLevel(secondLevel)

	tagCandle := &shared.Candlestick{
		Open:   float64(3),
		Close:  float64(4),
		High:   float64(10),
		Low:    float64(1),
		Volume: float64(1),
	}

	// Ensure the market can check whether a candle tags a level.
	isTagged := mkt.tagged(level, tagCandle)
	assert.True(t, isTagged)

	// Ensure an invalidated level cannot be tagged.
	invalidTag := mkt.tagged(invalidLevel, tagCandle)
	assert.False(t, invalidTag)

	// Ensure a tagged tracked level starts the price data request process.
	mkt.Update(tagCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), true)

	// Ensure the market tracks previous and current candle used in updating it.
	assert.Equal(t, mkt.candleSnapshot.Last(), tagCandle)

	// Ensure tagged levels can be filtered from a market.
	taggedLevels := mkt.FilterTaggedLevels(tagCandle)
	assert.Equal(t, len(taggedLevels), 2)

	// Ensure 3 updates after a level is tagged the market signals a price data request.
	for idx := range 3 {
		candle := &shared.Candlestick{
			Open:   float64(4 + idx),
			Close:  float64(6 + idx),
			High:   float64(8 + idx),
			Low:    float64(4 + idx),
			Volume: float64(2 + idx),
		}
		mkt.Update(candle)
	}

	assert.True(t, mkt.RequestingPriceData())

	// Ensure level reactions can be generated from a market.
	data := []*shared.Candlestick{
		{
			Open:   float64(3),
			Close:  float64(4),
			High:   float64(4),
			Low:    float64(1),
			Volume: float64(1),
		},
		{
			Open:   float64(4),
			Close:  float64(6),
			High:   float64(7),
			Low:    float64(3),
			Volume: float64(2),
		},
		{
			Open:   float64(6),
			Close:  float64(8),
			High:   float64(9),
			Low:    float64(5),
			Volume: float64(2),
		},
		{
			Open:   float64(8),
			Close:  float64(10),
			High:   float64(11),
			Low:    float64(7),
			Volume: float64(2),
		},
	}

	reactions, err := mkt.GenerateLevelReactions(data)
	assert.NoError(t, err)
	assert.Equal(t, len(reactions), 2)
	assert.Equal(t, reactions[0].Reaction, shared.Reversal)
	assert.Equal(t, reactions[1].Reaction, shared.Reversal)
	assert.Equal(t, reactions[0].PriceMovement,
		[]shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above})
	assert.Equal(t, reactions[1].PriceMovement,
		[]shared.Movement{shared.Below, shared.Below, shared.Below, shared.Equal})

	// Ensure the price data state of a market can be reset.
	mkt.ResetPriceDataState()
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(0))
}
