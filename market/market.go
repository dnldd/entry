package market

import (
	"fmt"
	"sync/atomic"

	"github.com/dnldd/entry/indicator"
	"github.com/dnldd/entry/shared"
)

type MarketConfig struct {
	// Market is the name of the tracked market.
	Market string
	// SignalLevel relays the provided level signal for processing.
	SignalLevel func(signal shared.LevelSignal)
	// SignalResistance relays the provided resistance.
	SignalResistance func(price float64)
}

// Market tracks the metadata of a market.
type Market struct {
	cfg             *MarketConfig
	candleSnapshot  *CandlestickSnapshot
	sessionSnapshot *SessionSnapshot
	vwap            *indicator.VWAP
	caughtUp        atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig) (*Market, error) {
	sessionsSnapshot, err := NewSessionSnapshot()
	if err != nil {
		return nil, err
	}

	mkt := &Market{
		cfg:             cfg,
		candleSnapshot:  NewCandlestickSnapshot(),
		sessionSnapshot: sessionsSnapshot,
		vwap:            indicator.NewVWAP(cfg.Market, shared.FiveMinute),
	}

	return mkt, nil
}

// SetCaughtUpStatus updates the caught up status of the provided market.
func (m *Market) SetCaughtUpStatus(status bool) {
	m.caughtUp.Store(status)
}

// CaughtUp returns whether the provided market has caught up on market data.
func (m *Market) CaughtUp() bool {
	return m.caughtUp.Load()
}

// Update processes incoming market data for the provided market.
func (m *Market) Update(candle *shared.Candlestick) error {
	// opting to use the 5-minute timeframe for timeframe agnostic session high/low
	// and vwap calculations.
	if candle.Timeframe != shared.FiveMinute {
		// do nothing.
		return nil
	}

	m.candleSnapshot.Update(candle)
	changed, err := m.sessionSnapshot.SetCurrentSession()
	if err != nil {
		return fmt.Errorf("setting current session: %w", err)
	}

	m.sessionSnapshot.FetchCurrentSession().Update(candle)
	_, err = m.vwap.Update(candle)
	if err != nil {
		return err
	}

	if changed {
		// Fetch and send new high and low from completed sessions.
		high, low, err := m.sessionSnapshot.FetchLastSessionHighLow()
		if err != nil {
			return fmt.Errorf("fetching new levels: %w", err)
		}

		sessionHigh := shared.LevelSignal{
			Market: candle.Market,
			Price:  high,
		}
		m.cfg.SignalLevel(sessionHigh)

		sessionLow := shared.LevelSignal{
			Market: candle.Market,
			Price:  low,
		}
		m.cfg.SignalLevel(sessionLow)
	}

	return nil
}
