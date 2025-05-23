package priceaction

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestMarket(t *testing.T) {
	market := "^GSPC"
	vwap := shared.VWAP{Value: 8}

	// Ensure a market can be created.
	cfg := &MarketConfig{
		Market: market,
		RequestVWAP: func(request shared.VWAPRequest) {
			request.Response <- &vwap
		},
		FetchCaughtUpState: func(market string) (bool, error) {
			return true, nil
		},
		Logger: &log.Logger,
	}
	mkt, err := NewMarket(cfg)
	assert.NoError(t, err)
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.levelUpdateCounter.Load(), uint32(0))
	assert.Equal(t, mkt.requestingPriceData.Load(), false)
	// Ensure a market can be updated with price data.
	supportClose := float64(8)
	supportCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),
		Status: make(chan shared.StatusCode, 1),
	}

	resistanceClose := float64(9)

	mkt.Update(supportCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.levelUpdateCounter.Load(), uint32(0))
	assert.Equal(t, mkt.requestingPriceData.Load(), false)

	// Ensure levels can be added to the market.
	supportPrice := float64(2)
	level := shared.NewLevel(market, supportPrice, supportClose)
	mkt.AddLevel(level)

	invalidLevel := shared.NewLevel(market, supportPrice, supportClose)
	invalidLevel.Invalidated.Store(true)
	mkt.AddLevel(invalidLevel)

	resistancePrice := float64(10)
	secondLevel := shared.NewLevel(market, resistancePrice, resistanceClose)
	mkt.AddLevel(secondLevel)

	// Ensure imbalances can be addeed to the market.
	bullishImbalance := shared.Imbalance{High: float64(12), Low: float64(8), Sentiment: shared.Bullish}
	mkt.AddImbalance(&bullishImbalance)
	bearishImbalance := shared.Imbalance{High: float64(8), Low: float64(5), Sentiment: shared.Bearish}
	mkt.AddImbalance(&bearishImbalance)

	tagCandle := &shared.Candlestick{
		Open:   float64(3),
		Close:  float64(4),
		High:   float64(10),
		Low:    float64(1),
		Volume: float64(1),
		Status: make(chan shared.StatusCode, 1),
	}

	// Ensure the market can check whether a candle tags a level.
	isTagged := mkt.levelTagged(level, tagCandle)
	assert.True(t, isTagged)

	// Ensure an invalidated level cannot be tagged.
	invalidTag := mkt.levelTagged(invalidLevel, tagCandle)
	assert.False(t, invalidTag)

	// Ensure a tagged tracked level starts the price data request process.
	mkt.Update(tagCandle)
	assert.Equal(t, mkt.taggedLevels.Load(), true)

	// Ensure tagged levels can be filtered from a market.
	taggedLevels := mkt.filterTaggedLevels(tagCandle)
	assert.Equal(t, len(taggedLevels), 2)

	// Ensure 3 updates after a level,vwap or imbalance is tagged the market signals for the
	// corresponding data request.
	for idx := range 3 {
		candle := &shared.Candlestick{
			Open:   float64(4 + idx),
			Close:  float64(6 + idx),
			High:   float64(8 + idx),
			Low:    float64(4 + idx),
			Volume: float64(2 + idx),
			Status: make(chan shared.StatusCode, 1),
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
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(4),
			Close:  float64(6),
			High:   float64(7),
			Low:    float64(3),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(6),
			Close:  float64(8),
			High:   float64(9),
			Low:    float64(5),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(8),
			Close:  float64(10),
			High:   float64(11),
			Low:    float64(7),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
	}

	reactions, err := mkt.GenerateReactionsAtTaggedLevels(data)
	assert.NoError(t, err)
	assert.Equal(t, len(reactions), 1)
	assert.Equal(t, reactions[0].Reaction, shared.Reversal)
	assert.Equal(t, reactions[0].PriceMovement,
		[]shared.PriceMovement{shared.Above, shared.Above, shared.Above, shared.Above})

	// Ensure the price data state of a market can be reset.
	mkt.ResetPriceDataState()
	assert.Equal(t, mkt.taggedLevels.Load(), false)
	assert.Equal(t, mkt.levelUpdateCounter.Load(), uint32(0))

	assert.True(t, mkt.RequestingVWAPData())

	// Ensure the vwap data state of a market can be reset.
	mkt.ResetVWAPDataState()
	assert.Equal(t, mkt.taggedVWAP.Load(), false)
	assert.Equal(t, mkt.vwapUpdateCounter.Load(), uint32(0))

	assert.True(t, mkt.RequestingImbalanceData())

	// Ensure imbalance reactions can be generated from a market.
	imbData := []*shared.Candlestick{
		{
			Open:   float64(3),
			Close:  float64(4),
			High:   float64(10),
			Low:    float64(1),
			Volume: float64(1),
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(4),
			Close:  float64(6),
			High:   float64(7),
			Low:    float64(3),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(6),
			Close:  float64(7.5),
			High:   float64(9),
			Low:    float64(5),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
		{
			Open:   float64(7.5),
			Close:  float64(10),
			High:   float64(11),
			Low:    float64(7),
			Volume: float64(2),
			Status: make(chan shared.StatusCode, 1),
		},
	}

	rctns, err := mkt.GenerateReactionsAtTaggedImbalances(imbData)
	assert.NoError(t, err)
	assert.Equal(t, len(rctns), 1)
	assert.Equal(t, rctns[0].Reaction, shared.Break)
	assert.Equal(t, rctns[0].PriceMovement,
		[]shared.PriceMovement{shared.Below, shared.Below, shared.Below, shared.Above})

	// Ensure the imbalance data state of a market can be reset.
	mkt.ResetImbalanceDataState()
	assert.Equal(t, mkt.taggedImbalance.Load(), false)
	assert.Equal(t, mkt.imbalanceUpdateCounter.Load(), uint32(0))
}
