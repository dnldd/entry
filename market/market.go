package market

import (
	"fmt"

	"github.com/dnldd/entry/indicator"
	"github.com/dnldd/entry/shared"
)

type MarketConfig struct {
	// Market is the name of the tracked market.
	Market string
	// SignalSupport relays the provided support.
	SignalSupport func(price float64)
	// SignalResistance relays the provided resistance.
	SignalResistance func(price float64)
}

// Market tracks the metadata of a market.
type Market struct {
	cfg             *MarketConfig
	candleSnapshot  *CandlestickSnapshot
	sessionSnapshot *SessionSnapshot
	vwap            *indicator.VWAP
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

		m.cfg.SignalResistance(high)
		m.cfg.SignalSupport(low)
	}

	return nil
}
