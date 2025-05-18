package priceaction

import (
	"fmt"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

const (
	// smallSnapshotSize is the buffer size for snapshots considered to hold a smaller set.
	smallSnapshotSize = 8
)

type MarketConfig struct {
	// Market is the name of the tracked market.
	Market string
	// RequestVWAPData relays the provided vwap request for processing.
	RequestVWAPData func(request shared.VWAPDataRequest)
	// RequestVWAP relays the provided vwap request for processing.
	RequestVWAP func(request shared.VWAPRequest)
	// FetchCaughtUpState returns the caught up status of the provided market.
	FetchCaughtUpState func(market string) (bool, error)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Market represents all the the price action data related to a market.
type Market struct {
	cfg                 *MarketConfig
	levelSnapshot       *LevelSnapshot
	taggedLevels        atomic.Bool
	taggedVWAP          atomic.Bool
	levelUpdateCounter  atomic.Uint32
	vwapUpdateCounter   atomic.Uint32
	requestingPriceData atomic.Bool
	requestingVWAPData  atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig) (*Market, error) {
	levelSnapshot, err := NewLevelSnapshot(levelSnapshotSize)
	if err != nil {
		return nil, fmt.Errorf("creating level snapshot: %v", err)
	}

	mgr := &Market{
		cfg:           cfg,
		levelSnapshot: levelSnapshot,
	}

	return mgr, nil
}

// evaluateTaggedLevels checks whether levels have been tagged by current price action. If confirmed
// a price data request is signalled after a brief interval of updates.
func (m *Market) evaluateTaggedLevels(candle *shared.Candlestick) {
	filteredLevels := m.FilterTaggedLevels(candle)

	switch {
	case len(filteredLevels) > 0 && !m.taggedLevels.Load() && m.levelUpdateCounter.Load() == 0:
		// Set the tagged levels flag to true if there is no pending price data request.
		m.taggedLevels.Store(true)

	case m.taggedLevels.Load() && m.levelUpdateCounter.Load() < shared.MaxPriceDataRequestInterval:
		// Increment the update counter while its below the price data request interval and set
		// the price data request flag to true once the data request interval is reached.
		counter := m.levelUpdateCounter.Add(1)
		if counter == shared.MaxPriceDataRequestInterval && !m.requestingPriceData.Load() {
			// NB: once a level is tagged it will take MaxPriceDataRequestInterval worth (3) of
			// market updates before the market signals requesting for price data.
			m.requestingPriceData.Store(true)
		}
	}
}

// evaluateTaggedVWAP checks whether the current vwap is tagged by current price action. If confirmed
// a vwap data request is signalled after a brief interval of updates.
func (m *Market) evaluateTaggedVWAP(candle *shared.Candlestick, vwap *shared.VWAP) {
	vwapTagged := m.vwapTagged(candle, vwap)

	switch {
	case vwapTagged && !m.taggedVWAP.Load() && m.vwapUpdateCounter.Load() == 0:
		// Set the tagged vwap flag to true if there is no pending vwap data request.
		m.taggedVWAP.Store(true)

	case m.taggedVWAP.Load() && m.vwapUpdateCounter.Load() < shared.MaxVWAPDataRequestInterval:
		// Increment the update counter while its below the vwap data request interval and set
		// the price data request flag to true once the data request interval is reached.
		counter := m.vwapUpdateCounter.Add(1)
		if counter == shared.MaxVWAPDataRequestInterval && !m.requestingPriceData.Load() {
			// NB: once the vwap is tagged it will take MaxVWAPDataRequestInterval worth (3) of
			// market updates before the market signals requesting for vwap data.
			m.requestingVWAPData.Store(true)
		}
	}
}

// Update processes the provided market candlestick data.
func (m *Market) Update(candle *shared.Candlestick) {
	m.levelSnapshot.Update(candle)

	caughtUp, err := m.cfg.FetchCaughtUpState(m.cfg.Market)
	if err != nil {
		m.cfg.Logger.Error().Msgf("fetching %s caught up state: %v", m.cfg.Market, err)
	}

	// Only evaluate vwap tags when the market is confirmed to be caught up.
	if caughtUp {
		m.evaluateTaggedLevels(candle)

		// Fetch the vwap corresponding to the update candle.
		var vwap *shared.VWAP
		req := shared.NewVWAPRequest(m.cfg.Market, candle.Date, candle.Timeframe)
		m.cfg.RequestVWAP(*req)
		select {
		case vwap = <-req.Response:
		case <-time.After(shared.TimeoutDuration * 4):
			m.cfg.Logger.Error().Msgf("timed out waiting for current vwap response")
			return
		}

		m.evaluateTaggedVWAP(candle, vwap)
	}
}

// RequestingPriceData indicates whether the provided market is requesting price data.
func (m *Market) RequestingPriceData() bool {
	return m.requestingPriceData.Load()
}

// RequestingVWAPData indicates whether the provided market is requesting vwap data.
func (m *Market) RequestingVWAPData() bool {
	return m.requestingVWAPData.Load()
}

// AddLevel adds the provided level to the market's level snapshot.
func (m *Market) AddLevel(level *shared.Level) {
	m.levelSnapshot.Add(level)
}

// vwaptagged checks whether the provided vwap was tagged by the provided candlestick.
func (m *Market) vwapTagged(candle *shared.Candlestick, vwap *shared.VWAP) bool {
	var kind shared.LevelKind
	switch {
	case vwap.Value > candle.Close:
		kind = shared.Resistance
	case vwap.Value < candle.Close:
		kind = shared.Support
	}

	switch kind {
	case shared.Support:
		if candle.Low <= vwap.Value {
			return true
		}
	case shared.Resistance:
		if candle.High >= vwap.Value {
			return true
		}
	}

	return false
}

// taggedLevel checks whether the provided level was tagged by the provided candlestick.
func (m *Market) taggedLevel(level *shared.Level, candle *shared.Candlestick) bool {
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
	taggedLevels := m.levelSnapshot.Filter(candle, m.taggedLevel)
	return taggedLevels
}

// GenerateReactionsAtTaggedLevels generates reactions for all levels tagged by the first of the provided market candlestick data.
func (m *Market) GenerateReactionsAtTaggedLevels(data []*shared.Candlestick) ([]*shared.ReactionAtLevel, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be an empty slice")
	}
	// Fetch all levels tagged by the first of the provided market candlestick data.
	firstCandle := data[0]
	taggedSet := make([]*shared.Level, 0)

	filtered := m.FilterTaggedLevels(firstCandle)
	taggedSet = append(taggedSet, filtered...)

	// Create the associated price level reactions for all tagged levels.
	reactions := make([]*shared.ReactionAtLevel, len(taggedSet))
	for idx := range taggedSet {
		taggedLevel := taggedSet[idx]
		reaction, err := shared.NewReactionAtLevel(m.cfg.Market, taggedLevel, data)
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
	m.levelUpdateCounter.Store(0)
	m.requestingPriceData.Store(false)
}

// ResetVWAPDataState resets the flags and counters associated with vwap data state for the market.
func (m *Market) ResetVWAPDataState() {
	m.taggedVWAP.Store(false)
	m.vwapUpdateCounter.Store(0)
	m.requestingVWAPData.Store(false)
}
