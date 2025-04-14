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
	assert.Equal(t, mkt.currentCandle.Load(), nil)
	assert.Equal(t, mkt.previousCandle.Load(), nil)

	// Ensure a market can be updated with price data.
	firstCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
	}

	mkt.Update(firstCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(0))
	assert.Equal(t, mkt.requestingPriceData.Load(), false)
	assert.NotEqual(t, mkt.currentCandle.Load(), nil)
	assert.Equal(t, mkt.previousCandle.Load(), nil)

	// Ensure levels can be added to the market.
	price := float64(2)
	level := shared.NewLevel(market, price, firstCandle)
	mkt.AddLevel(level)

	secondCandle := &shared.Candlestick{
		Open:   float64(3),
		Close:  float64(4),
		High:   float64(4),
		Low:    float64(1),
		Volume: float64(1),
	}

	// Ensure the market can check whether a candle tags a level.
	isTagged := mkt.tagged(level, secondCandle)
	assert.True(t, isTagged)

	// Ensure a tagged tracked level starts the price data request process.
	mkt.Update(secondCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), true)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(1))

	// Ensure the market tracks previous and current candle used it updating it.
	assert.Equal(t, mkt.previousCandle.Load(), firstCandle)
	assert.Equal(t, mkt.currentCandle.Load(), secondCandle)

	// Ensure the previous and current candle can be fetched from the market.
	fetchedCandle := mkt.FetchPreviousCandle()
	assert.Equal(t, fetchedCandle, firstCandle)
	fetchedCandle = mkt.FetchCurrentCandle()
	assert.Equal(t, fetchedCandle, secondCandle)

	// Ensure tagged levels can be filtered from a market.
	taggedLevels := mkt.FilterTaggedLevels(secondCandle)
	assert.Equal(t, len(taggedLevels), 1)

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
	assert.Equal(t, len(reactions), 1)
	assert.Equal(t, reactions[0].Reaction, shared.Reversal)
	assert.Equal(t, reactions[0].PriceMovement,
		[]shared.Movement{shared.Above, shared.Above, shared.Above, shared.Above})

	// Ensure the status of a price request can be fetched from a market.
	status := mkt.RequestingPriceData()
	assert.Equal(t, status, false)

	// Ensure the price data state of a market can be reset.
	mkt.ResetPriceDataState()
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.updateCounter.Load(), uint32(0))
}
