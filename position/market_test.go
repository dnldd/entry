package position

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestMarket(t *testing.T) {
	// Ensure a market can be created.
	market := "^GSPC"
	mkt := NewMarket(market)

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure a market can track positions.
	entrySignal := &shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
		CreatedOn: now,
	}

	pos, err := NewPosition(entrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)

	// Ensure tracked positions can be updated.
	candle := &shared.Candlestick{
		Open:   12,
		Close:  15,
		High:   16,
		Low:    8,
		Volume: 3,
		Date:   now,

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	err = mkt.Update(candle)
	assert.NoError(t, err)
	assert.Equal(t, MarketStatus(mkt.status.Load()), LongInclined)

	// Ensure a tracked position can be removed.
	mkt.RemovePosition(pos.ID)
	assert.Equal(t, MarketStatus(mkt.status.Load()), Neutral)
}
