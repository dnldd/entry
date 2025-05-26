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
	cfg                     *MarketConfig
	levelSnapshot           *shared.LevelSnapshot
	imbalanceSnapshot       *shared.ImbalanceSnapshot
	taggedLevels            atomic.Bool
	taggedVWAP              atomic.Bool
	taggedImbalance         atomic.Bool
	levelUpdateCounter      atomic.Uint32
	vwapUpdateCounter       atomic.Uint32
	imbalanceUpdateCounter  atomic.Uint32
	requestingPriceData     atomic.Bool
	requestingVWAPData      atomic.Bool
	requestingImbalanceData atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig) (*Market, error) {
	levelSnapshot, err := shared.NewLevelSnapshot(shared.LevelSnapshotSize)
	if err != nil {
		return nil, fmt.Errorf("creating level snapshot: %v", err)
	}

	imbalanceSnapshot, err := shared.NewImbalanceSnapshot(shared.ImbalanceSnapshotSize)
	if err != nil {
		return nil, fmt.Errorf("creating imbalance snapshot: %v", err)
	}

	mgr := &Market{
		cfg:               cfg,
		levelSnapshot:     levelSnapshot,
		imbalanceSnapshot: imbalanceSnapshot,
	}

	return mgr, nil
}

// evaluateTaggedLevels checks whether levels have been tagged by current price action. If confirmed
// a price data request is signalled after a brief interval of updates.
func (m *Market) evaluateTaggedLevels(candle *shared.Candlestick) {
	filteredLevels := m.filterTaggedLevels(candle)
	taggedLevels := m.taggedLevels.Load()
	levelUpdateCounter := m.levelUpdateCounter.Load()
	requestingPriceData := m.requestingPriceData.Load()

	switch {
	case len(filteredLevels) > 0 && !taggedLevels && levelUpdateCounter == 0:
		// Set the tagged levels flag to true if there is no pending price data request.
		m.taggedLevels.Store(true)

	case taggedLevels && levelUpdateCounter < shared.MaxPriceDataRequestInterval:
		// Increment the update counter while its below the price data request interval and set
		// the price data request flag to true once the data request interval is reached.
		counter := m.levelUpdateCounter.Add(1)
		if counter == shared.MaxPriceDataRequestInterval && !requestingPriceData {
			// NB: once a level is tagged it will take MaxPriceDataRequestInterval worth (3) of
			// market updates before the market signals requesting for price data.
			m.requestingPriceData.Store(true)
		}
	}
}

// evaluateTaggedVWAP checks whether the current vwap is tagged by current price action. If confirmed
// a vwap data request is signalled after a brief interval of updates.
func (m *Market) evaluateTaggedVWAP(candle *shared.Candlestick, vwap *shared.VWAP) {
	vwapTagged := m.vwapTagged(vwap, candle)
	taggedVWAP := m.taggedVWAP.Load()
	requestingPriceData := m.requestingPriceData.Load()
	vwapUpdateCounter := m.vwapUpdateCounter.Load()

	switch {
	case vwapTagged && !taggedVWAP && vwapUpdateCounter == 0:
		// Set the tagged vwap flag to true if there is no pending vwap data request.
		m.taggedVWAP.Store(true)

	case taggedVWAP && vwapUpdateCounter < shared.MaxVWAPDataRequestInterval:
		// Increment the update counter while its below the vwap data request interval and set
		// the price data request flag to true once the data request interval is reached.
		counter := m.vwapUpdateCounter.Add(1)
		if counter == shared.MaxVWAPDataRequestInterval && !requestingPriceData {
			// NB: once the vwap is tagged it will take MaxVWAPDataRequestInterval worth (3) of
			// market updates before the market signals requesting for vwap data.
			m.requestingVWAPData.Store(true)
		}
	}
}

// evaluateTaggedImbalances checks whether imbalances are tagged by current price action. If confirmed
// an imbalance data request is signalled after a brief interval of updates.
func (m *Market) evaluateTaggedImbalances(candle *shared.Candlestick) {
	filteredImbalances := m.filterTaggedImbalances(candle)
	taggedImbalance := m.taggedImbalance.Load()
	imbalanceUpdateCounter := m.imbalanceUpdateCounter.Load()
	requestingImbalanceData := m.requestingImbalanceData.Load()

	switch {
	case len(filteredImbalances) > 0 && !taggedImbalance && imbalanceUpdateCounter == 0:
		// Set the tagged imbalance flag to true if there is no pending imbalance data request.
		m.taggedImbalance.Store(true)

	case taggedImbalance && imbalanceUpdateCounter < shared.MaxVWAPDataRequestInterval:
		// Increment the update counter while its below the imbalance data request interval and set
		// the price data request flag to true once the data request interval is reached.
		counter := m.imbalanceUpdateCounter.Add(1)
		if counter == shared.MaxImbalanceDataRequestInterval && !requestingImbalanceData {
			// NB: once the imbalance is tagged it will take MaxImbalanceDataRequestInterval worth (3) of
			// market updates before the market signals requesting for imbalance data.
			m.requestingImbalanceData.Store(true)
		}
	}
}

// Update processes the provided market candlestick data.
func (m *Market) Update(candle *shared.Candlestick) {
	m.levelSnapshot.Update(candle)
	m.imbalanceSnapshot.Update(candle)

	caughtUp, err := m.cfg.FetchCaughtUpState(m.cfg.Market)
	if err != nil {
		m.cfg.Logger.Error().Msgf("fetching %s caught up state: %v", m.cfg.Market, err)
	}

	// Only evaluate vwap and imbalance tags when the market is confirmed to be caught up.
	if caughtUp {
		m.evaluateTaggedLevels(candle)
		m.evaluateTaggedImbalances(candle)

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

// RequestingVWAPData indicates whether the provided market is requesting imbalance data.
func (m *Market) RequestingImbalanceData() bool {
	return m.requestingImbalanceData.Load()
}

// AddLevel adds the provided level to the market's level snapshot.
func (m *Market) AddLevel(level *shared.Level) {
	m.levelSnapshot.Add(level)
}

// AddImbalance adds the provided level to the market's level snapshot.
func (m *Market) AddImbalance(imb *shared.Imbalance) {
	m.imbalanceSnapshot.Add(imb)
}

// vwaptagged checks whether the provided vwap was tagged by the provided candlestick.
func (m *Market) vwapTagged(vwap *shared.VWAP, candle *shared.Candlestick) bool {
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

// levelTagged checks whether the provided level was tagged by the provided candlestick.
func (m *Market) levelTagged(level *shared.Level, candle *shared.Candlestick) bool {
	invalidated := level.Invalidated.Load()
	if invalidated {
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

// imbalanceTagged determines whether the provided imbalance was tagged by the provided candlestick.
func (m *Market) imbalanceTagged(imb *shared.Imbalance, candle *shared.Candlestick) bool {
	invalidated := imb.Invalidated.Load()
	if invalidated {
		return false
	}

	switch imb.Sentiment {
	case shared.Bullish:
		if candle.Low <= imb.High {
			return true
		}
	case shared.Bearish:
		if candle.High >= imb.Low {
			return true
		}
	}

	return false
}

// filterTaggedLevels filters levels tagged by the provided candle.
func (m *Market) filterTaggedLevels(candle *shared.Candlestick) []*shared.Level {
	taggedLevels := m.levelSnapshot.Filter(candle, m.levelTagged)
	return taggedLevels
}

// filterTaggedImbalances filters imbalances tagged by the provided candle.
func (m *Market) filterTaggedImbalances(candle *shared.Candlestick) []*shared.Imbalance {
	taggedImbalances := m.imbalanceSnapshot.Filter(candle, m.imbalanceTagged)
	return taggedImbalances
}

// GenerateReactionsAtTaggedLevels generates reactions for all levels tagged by the first of the provided market candlestick data.
func (m *Market) GenerateReactionsAtTaggedLevels(data []*shared.Candlestick) ([]*shared.ReactionAtLevel, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be an empty slice")
	}
	// Fetch all levels tagged by the first of the provided market candlestick data.
	firstCandle := data[0]
	taggedSet := make([]*shared.Level, 0)

	filtered := m.filterTaggedLevels(firstCandle)
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

// GenerateReactionsAtTaggedImbalances generates reactions for all levels tagged by the first of the provided market candlestick data.
func (m *Market) GenerateReactionsAtTaggedImbalances(data []*shared.Candlestick) ([]*shared.ReactionAtImbalance, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be an empty slice")
	}
	// Fetch all imbalances tagged by the first of the provided market candlestick data.
	firstCandle := data[0]
	taggedSet := make([]*shared.Imbalance, 0)

	filtered := m.filterTaggedImbalances(firstCandle)
	taggedSet = append(taggedSet, filtered...)

	// Create the associated price imbalance reactions for all tagged levels.
	reactions := make([]*shared.ReactionAtImbalance, len(taggedSet))
	for idx := range taggedSet {
		taggedImbalance := taggedSet[idx]
		reaction, err := shared.NewReactionAtImbalance(m.cfg.Market, taggedImbalance, data)
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

// ResetImbalanceDataState resets the flags and counters associated with imbalance data state for the market.
func (m *Market) ResetImbalanceDataState() {
	m.taggedImbalance.Store(false)
	m.imbalanceUpdateCounter.Store(0)
	m.requestingImbalanceData.Store(false)
}
