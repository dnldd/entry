package priceaction

import (
	"fmt"
	"sync/atomic"

	"github.com/dnldd/entry/shared"
)

const (
	// maxPriceDataRequestInterval is the maximum update intervals to wait before
	// triggering a price data request.
	maxPriceDataRequestInterval = 3
	// smallSnapshotSize is the buffer size for snapshots considered to hold a smaller set.
	smallSnapshotSize = 8
)

// Market represents all the the price action data related to a market.
type Market struct {
	market              string
	levelSnapshot       *LevelSnapshot
	candleSnapshot      *shared.CandlestickSnapshot
	taggedLevels        atomic.Bool
	updateCounter       atomic.Uint32
	requestingPriceData atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(market string) (*Market, error) {
	levelSnapshot, err := NewLevelSnapshot(levelSnapshotSize)
	if err != nil {
		return nil, fmt.Errorf("creating level snapshot: %v", err)
	}

	candleSnapshot, err := shared.NewCandlestickSnapshot(smallSnapshotSize)
	if err != nil {
		return nil, fmt.Errorf("creating candle snapshot: %v", err)
	}

	mgr := &Market{
		market:         market,
		levelSnapshot:  levelSnapshot,
		candleSnapshot: candleSnapshot,
	}

	return mgr, nil
}

// FetchCurrentCandle fetches the current market candlestick.
func (m *Market) FetchCurrentCandle() *shared.Candlestick {
	return m.candleSnapshot.Last()
}

// UpdateCurrentCandle market's price action concepts .
func (m *Market) Update(candle *shared.Candlestick) {
	m.levelSnapshot.Update(candle)
	m.candleSnapshot.Update(candle)

	filteredLevels := m.FilterTaggedLevels(candle)
	hasTaggedLevels := m.taggedLevels.Load()
	updateCounter := m.updateCounter.Load()
	requestingPriceData := m.requestingPriceData.Load()

	switch {
	case len(filteredLevels) > 0 && !hasTaggedLevels && updateCounter == 0:
		// Set the tagged levels flag to true if there is no pending price data request.
		m.taggedLevels.Store(true)
		m.updateCounter.Add(1)
	case hasTaggedLevels && updateCounter > 0 && updateCounter < maxPriceDataRequestInterval:
		// Increment the update counter while its below the price data request interval.
		m.updateCounter.Add(1)
	case hasTaggedLevels && updateCounter == maxPriceDataRequestInterval && !requestingPriceData:
		// Set the price data request flag to true once the data request interval is reached.
		m.requestingPriceData.Store(true)
	}
}

// RequestingPriceData indicates whether the provided market is requesting price data.
func (m *Market) RequestingPriceData() bool {
	return m.requestingPriceData.Load()
}

// AddLevel adds the provided level to the market's level snapshot.
func (m *Market) AddLevel(level *shared.Level) {
	m.levelSnapshot.Add(level)
}

// tagged checks whether the provided level is tagged by the provided candlestick.
func (m *Market) tagged(level *shared.Level, candle *shared.Candlestick) bool {
	if level.Invalidated.Load() {
		return false
	}

	switch level.Kind {
	case shared.Support:
		if candle.Low <= level.Price {
			return true
		}
	case shared.Resistance:
		if candle.High >= level.Price {
			return true
		}
	}

	return false
}

// FilterTaggedLevels filters levels tagged by the provided candle.
func (m *Market) FilterTaggedLevels(candle *shared.Candlestick) []*shared.Level {
	taggedLevels := m.levelSnapshot.Filter(candle, m.tagged)
	return taggedLevels
}

// GenerateLevelReactions generates level reactions for all levels tagged by the provided
// market candlestick data.
func (m *Market) GenerateLevelReactions(data []*shared.Candlestick) ([]*shared.LevelReaction, error) {
	// Fetch all levels tagged by the provided price data.
	taggedSet := make([]*shared.Level, 0)
	for idx := range data {
		candle := data[idx]
		filtered := m.FilterTaggedLevels(candle)
		taggedSet = append(taggedSet, filtered...)
	}

	// Create the associated price level reactions for all tagged levels.
	reactions := make([]*shared.LevelReaction, len(taggedSet))
	for idx := range taggedSet {
		taggedLevel := taggedSet[idx]
		reaction, err := shared.NewLevelReaction(m.market, taggedLevel, data)
		if err != nil {
			return nil, err
		}
		reactions[idx] = reaction
	}

	return reactions, nil
}

// ResetPriceDataState resets the flags and counters associated with price data state for the market.
func (m *Market) ResetPriceDataState() {
	m.taggedLevels.Store(false)
	m.updateCounter.Store(0)
	m.requestingPriceData.Store(false)
}
