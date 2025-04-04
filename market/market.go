package market

import (
	"fmt"
	"slices"
	"time"

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
	cfg               *MarketConfig
	sessions          []*Session
	currentSessionIdx int
	vwap              *indicator.VWAP
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig) (*Market, error) {
	mkt := &Market{
		cfg:      cfg,
		sessions: make([]*Session, 0, maxSessions),
		vwap:     indicator.NewVWAP(cfg.Market, shared.FiveMinute),
	}

	err := mkt.AddSessions()
	if err != nil {
		return nil, fmt.Errorf("adding sessions: %w", err)
	}

	_, err = mkt.setCurrentSession()
	if err != nil {
		return nil, fmt.Errorf("setting current session: %w", err)
	}

	return mkt, nil
}

// AddSessions adds the next set of sessions (london, new york and asia) to the market.
func (m *Market) AddSessions() error {
	// If at capacity with tracked sessions, remove the oldest three.
	if len(m.sessions) == int(maxSessions) {
		m.sessions = slices.Delete(m.sessions, 0, 3)
	}

	londonSession, err := NewSession(london, londonOpen, londonClose)
	if err != nil {
		return fmt.Errorf("creating london session: %w", err)
	}

	m.sessions = append(m.sessions, londonSession)

	newYorkSession, err := NewSession(newYork, newYorkOpen, newYorkClose)
	if err != nil {
		return fmt.Errorf("creating new york session: %w", err)
	}

	m.sessions = append(m.sessions, newYorkSession)

	asianSession, err := NewSession(asia, asiaOpen, asiaClose)
	if err != nil {
		return fmt.Errorf("creating asian session: %w", err)
	}

	m.sessions = append(m.sessions, asianSession)

	return nil
}

// setCurrentSession sets the current session.
func (m *Market) setCurrentSession() (bool, error) {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return false, err
	}

	// Set the current session.
	var set bool
	var changed bool
	prev := m.currentSessionIdx
	for idx := range m.sessions {
		if m.sessions[idx].IsCurrentSession(now) {
			m.currentSessionIdx = idx
			set = true
			if prev != idx {
				// The changed flag indicates there has been a session change.
				changed = true
			}
			break
		}
	}

	// If the current session is not set then the market is closed and  current time is
	// approaching asian session. Preemptively set the asian session.
	if !set {
		m.currentSessionIdx = len(m.sessions) - 1
	}

	return changed, nil
}

// fetchLastSessionHighLow fetches newly generated levels from the previously completed session.
func (m *Market) fetchLastSessionHighLow() (float64, float64, error) {
	if m.currentSessionIdx == 0 {
		// There is no previous completed session.
		return 0, 0, fmt.Errorf("no completed previous session available")
	}

	previousSession := m.sessions[m.currentSessionIdx-1]

	return previousSession.High, previousSession.Low, nil
}

// Update processes incoming market data for the provided market.
func (m *Market) Update(candle *shared.Candlestick) error {
	// opting to use the 5-minute timeframe for timeframe agnostic session high/low
	// and vwap calculations.
	if candle.Timeframe != shared.FiveMinute {
		// do nothing.
		return nil
	}

	changed, err := m.setCurrentSession()
	if err != nil {
		return fmt.Errorf("setting current session: %w", err)
	}

	m.sessions[m.currentSessionIdx].Update(candle)
	_, err = m.vwap.Update(candle)
	if err != nil {
		return err
	}

	if changed {
		// Fetch and send new high and low from completed sessions.
		high, low, err := m.fetchLastSessionHighLow()
		if err != nil {
			return fmt.Errorf("fetching new levels: %w", err)
		}

		m.cfg.SignalResistance(high)
		m.cfg.SignalSupport(low)
	}

	return nil
}

// FetchLastSessionOpen returns the last session open.
func (m *Market) FetchLastSessionOpen() time.Time {
	var open time.Time
	if m.currentSessionIdx == 0 {
		// There is no last session, set the open to the current one.
		open = m.sessions[m.currentSessionIdx].Open
	}

	open = m.sessions[m.currentSessionIdx-1].Open

	return open
}
