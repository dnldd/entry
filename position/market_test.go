package position

import (
	"os"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestMarket(t *testing.T) {
	// Ensure a market can be created.
	market := "^GSPC"

	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &MarketConfig{
		Market:       market,
		JobScheduler: gocron.NewScheduler(loc),
		Logger:       &log.Logger,
	}
	mkt, err := NewMarket(cfg)
	assert.NoError(t, err)

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	oneYearAgo := now.AddDate(-1, 0, 0)

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
		CreatedOn: oneYearAgo,
		Status:    make(chan shared.StatusCode, 1),
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
		CreatedOn: oneYearAgo,
		Status:    make(chan shared.StatusCode, 1),
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
		Open:      12,
		Close:     15,
		High:      16,
		Low:       8,
		Volume:    3,
		Date:      oneYearAgo,
		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mkt.Update(candle)
	assert.NoError(t, err)
	assert.Equal(t, shared.MarketSkew(mkt.skew.Load()), shared.LongSkewed)

	// Ensure once a markets skew is set, tracking positions of the opposite.
	// skew returns an error.
	shortEntrySignal := &shared.EntrySignal{
		Market:    market,
		Timeframe: shared.FiveMinute,
		Direction: shared.Short,
		Price:     10,
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		StopLoss:  12,
		CreatedOn: oneYearAgo,
		Status:    make(chan shared.StatusCode, 1),
	}

	pos, err = NewPosition(shortEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.Error(t, err)

	// Ensure the market can persist its positions to file.
	filename, err := mkt.PersistPositionsCSV()
	assert.NoError(t, err)

	defer os.Remove(filename)

	// Ensure an exit signal for an unknown market returns an error.
	wrongMarketExitSignal := &shared.ExitSignal{
		Market:     "^AAPL",
		Timeframe:  shared.FiveMinute,
		Direction:  shared.Long,
		Price:      18,
		Reasons:    []shared.Reason{shared.BearishEngulfing},
		Confluence: 8,
		CreatedOn:  oneYearAgo,
		Status:     make(chan shared.StatusCode, 1),
	}

	closedPos, err := mkt.ClosePositions(wrongMarketExitSignal)
	assert.Error(t, err)

	// Ensure a tracked market position can be closed.
	longExitSignal := &shared.ExitSignal{
		Market:     market,
		Timeframe:  shared.FiveMinute,
		Direction:  shared.Long,
		Price:      18,
		Reasons:    []shared.Reason{shared.BearishEngulfing},
		Confluence: 8,
		CreatedOn:  oneYearAgo,
		Status:     make(chan shared.StatusCode, 1),
	}
	closedPos, err = mkt.ClosePositions(longExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, len(closedPos), 1)

	// Ensure the market's skew resets once all positions are closed.
	assert.Equal(t, shared.MarketSkew(mkt.skew.Load()), shared.NeutralSkew)

	// Ensure a market can track short positions.
	pos, err = NewPosition(shortEntrySignal)
	assert.NoError(t, err)

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)
	assert.Equal(t, shared.MarketSkew(mkt.skew.Load()), shared.ShortSkewed)

	// Ensure the market does not track duplicate short positions.
	mkt.positionMtx.RLock()
	sizeBefore = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	err = mkt.AddPosition(pos)
	assert.NoError(t, err)

	mkt.positionMtx.RLock()
	sizeAfter = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	// Ensure once a markets skew is set, tracking positions of the opposite
	// skew returns an error.
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
		CreatedOn:  oneYearAgo,
		Status:     make(chan shared.StatusCode, 1),
	}
	closedPos, err = mkt.ClosePositions(shortExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, len(closedPos), 1)

	// Ensure the market's skew resets once all positions are closed.
	assert.Equal(t, shared.MarketSkew(mkt.skew.Load()), shared.NeutralSkew)

	mkt.positionMtx.RLock()
	sizeBefore = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	// Ensure old closed positions can be purged.
	err = mkt.PurgeClosedPositionsJob()
	assert.NoError(t, err)

	mkt.positionMtx.RLock()
	sizeAfter = len(mkt.positions)
	mkt.positionMtx.RUnlock()

	assert.NotEqual(t, sizeBefore, sizeAfter)

}
