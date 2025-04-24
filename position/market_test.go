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

	// Ensure adding a nil position returns an error.
	err = mkt.AddPosition(nil)
	assert.Error(t, err)

	// Ensure adding an untracked market positions returns an error.
	untrackedMarketEntrySignal := &shared.EntrySignal{
		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
		CreatedOn: now,
	}

	pos, err := NewPosition(untrackedMarketEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.Error(t, err)

	// Ensure a market can track long positions.
	longEntrySignal := &shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
		CreatedOn: now,
	}

	pos, err = NewPosition(longEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)

	// Ensure the market does not track duplicate long positions
	mkt.positionMtx.RLock()
	sizeBefore := len(mkt.positions)
	mkt.positionMtx.RUnlock()

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)

	mkt.positionMtx.RLock()
	sizeAfter := len(mkt.positions)
	mkt.positionMtx.RUnlock()

	assert.Equal(t, sizeBefore, sizeAfter)

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
	assert.Equal(t, shared.MarketStatus(mkt.status.Load()), shared.LongInclined)

	// Ensure once a markets direction inclination is set, tracking positions of the opposite
	// direction inclination returns an error.
	shortEntrySignal := &shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Short,
		Price:     10,
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		StopLoss:  12,
		CreatedOn: now,
	}

	pos, err = NewPosition(shortEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.Error(t, err)

	// Ensure a closing tracked market posions an exit signal for another market returns an error.
	wrongMarketExitSignal := &shared.ExitSignal{
		Market:     "^AAPL",
		Timeframe:  shared.FiveMinute,
		Direction:  shared.Long,
		Price:      18,
		Reasons:    []shared.Reason{shared.BearishEngulfing},
		Confluence: 8,
		CreatedOn:  now,
	}

	closedPos, err := mkt.ClosePositions(wrongMarketExitSignal)
	assert.Error(t, err)

	// Ensure a tracked market positions can be closed.
	longExitSignal := &shared.ExitSignal{
		Market:     market,
		Timeframe:  shared.FiveMinute,
		Direction:  shared.Long,
		Price:      18,
		Reasons:    []shared.Reason{shared.BearishEngulfing},
		Confluence: 8,
		CreatedOn:  now,
	}
	closedPos, err = mkt.ClosePositions(longExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, len(closedPos), 1)

	// Ensure the market's direction inclination resets once all positions are closed.
	assert.Equal(t, shared.MarketStatus(mkt.status.Load()), shared.NeutralInclination)

	// Ensure a market can track short positions.
	pos, err = NewPosition(shortEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)
	assert.Equal(t, shared.MarketStatus(mkt.status.Load()), shared.ShortInclined)

	// Ensure the market does not track duplicate short positions.
	mkt.positionMtx.RLock()
	sizeBefore = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)

	mkt.positionMtx.RLock()
	sizeAfter = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	// Ensure once a markets direction inclination is set, tracking positions of the opposite
	// direction inclination returns an error.
	pos, err = NewPosition(longEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.Error(t, err)

	// Ensure a tracked market positions can be closed.
	shortExitSignal := &shared.ExitSignal{
		Market:     market,
		Timeframe:  shared.FiveMinute,
		Direction:  shared.Short,
		Price:      12,
		Reasons:    []shared.Reason{shared.BullishEngulfing},
		Confluence: 8,
		CreatedOn:  now,
	}
	closedPos, err = mkt.ClosePositions(shortExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, len(closedPos), 1)

	// Ensure the market's direction inclination resets once all positions are closed.
	assert.Equal(t, shared.MarketStatus(mkt.status.Load()), shared.NeutralInclination)
}
