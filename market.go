package main

import (
	"fmt"
	"slices"
	"time"
)

// Market tracks the metadata of a market.
type Market struct {
	Market   string
	Sessions []*Session
	VWAP     *VWAP
}

// NewMarket instantiates a new market.
func NewMarket(market string) *Market {
	return &Market{
		Market:   market,
		Sessions: make([]*Session, 0, maxSessions),
		VWAP:     NewVWAP(market, FiveMinute),
	}
}

// AddSessions adds the next set of sessions (asia, london & new york) to the market.
func (m *Market) AddSessions() error {
	// If at capacity with tracked sessions, remove the oldest three.
	if len(m.Sessions) == int(maxSessions) {
		m.Sessions = slices.Delete(m.Sessions, 0, 3)
	}

	asianSession, err := NewSession(asia, asiaOpen, asiaClose)
	if err != nil {
		return fmt.Errorf("creating asian session: %w", err)
	}

	m.Sessions = append(m.Sessions, asianSession)

	londonSession, err := NewSession(london, londonOpen, londonClose)
	if err != nil {
		return fmt.Errorf("creating london session: %w", err)
	}

	m.Sessions = append(m.Sessions, londonSession)

	newYorkSession, err := NewSession(newYork, newYorkOpen, newYorkClose)
	if err != nil {
		return fmt.Errorf("creating new york session: %w", err)
	}

	m.Sessions = append(m.Sessions, newYorkSession)

	return nil
}

// Update processes incoming market data for the provided market.
func (m *Market) Update(candle *Candlestick) error {
	// opting to use the 5-minute timeframe for timeframe agnostic session high/low
	// and vwap calculations.
	if candle.Timeframe != FiveMinute {
		// do nothing.
		return nil
	}

	now := time.Now().UTC()
	var currentSession *Session
	for idx := len(m.Sessions) - 1; idx > -1; idx-- {
		if m.Sessions[idx].IsCurrentSession(now) {
			currentSession = m.Sessions[idx]
			break
		}
	}

	currentSession.Update(candle)

	_, err := m.VWAP.Update(candle)
	if err != nil {
		return err
	}

	return nil
}

// FetchSessionLevels fetches eligible session highs and lows as levels.
func (m *Market) FetchSessionLevels() []float64 {
	levels := make([]float64, 0, len(m.Sessions)*2)
	for idx := range m.Sessions {
		if m.Sessions[idx].Matured {
			levels = append(levels, m.Sessions[idx].High, m.Sessions[idx].Low)
		}
	}

	return levels
}

// NB: filtering levels to identify highly probable ones will be left to the entry finder.
