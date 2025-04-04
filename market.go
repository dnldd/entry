package main

import (
	"fmt"
	"slices"
	"strings"
)

// Market tracks the metadata of a market.
type Market struct {
	Market         string
	Sessions       []*Session
	CurrentSession *Session
	VWAP           *VWAP

	SendLevel func(level Level)
}

// NewMarket initializes a new market.
func NewMarket(market string, f func(level Level)) *Market {
	return &Market{
		Market:    market,
		Sessions:  make([]*Session, 0, maxSessions),
		VWAP:      NewVWAP(market, FiveMinute),
		SendLevel: f,
	}
}

// AddSessions adds the next set of sessions (london, new york and asia) to the market.
func (m *Market) AddSessions() error {
	// If at capacity with tracked sessions, remove the oldest three.
	if len(m.Sessions) == int(maxSessions) {
		m.Sessions = slices.Delete(m.Sessions, 0, 3)
	}

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

	asianSession, err := NewSession(asia, asiaOpen, asiaClose)
	if err != nil {
		return fmt.Errorf("creating asian session: %w", err)
	}

	m.Sessions = append(m.Sessions, asianSession)

	return nil
}

// FetchNewLevels fetches newly generated levels from the previously completed session.
func (m *Market) FetchNewLevels() ([]*Level, error) {
	now, _, err := NewYorkTime()
	if err != nil {
		return nil, err
	}

	var completedSession *Session
	for idx := len(m.Sessions); idx > -1; idx-- {
		if m.Sessions[idx].IsCurrentSession(now) {
			// The completed session should be the next one after the current session
			// in the iteration.
			prevIdx := idx - 1
			completedSession = m.Sessions[prevIdx]
			break
		}
	}

	return []*Level{NewLevel(m.Market, completedSession.High), NewLevel(m.Market, completedSession.Low)}, nil
}

// Update processes incoming market data for the provided market.
func (m *Market) Update(candle *Candlestick) error {
	// opting to use the 5-minute timeframe for timeframe agnostic session high/low
	// and vwap calculations.
	if candle.Timeframe != FiveMinute {
		// do nothing.
		return nil
	}

	now, _, err := NewYorkTime()
	if err != nil {
		return fmt.Errorf("updating %s market: %s", m.Market, err)
	}

	var newSession bool
	for idx := len(m.Sessions) - 1; idx > -1; idx-- {
		if m.Sessions[idx].IsCurrentSession(now) {
			switch {
			case m.CurrentSession == nil:
				m.CurrentSession = m.Sessions[idx]
				break
			case strings.Compare(m.CurrentSession.Name, m.Sessions[idx].Name) != 0:
				// Flag a new session when there is a transition.
				newSession = true
				m.CurrentSession = m.Sessions[idx]
				break
			default:
				break
			}
		}
	}

	m.CurrentSession.Update(candle)
	_, err = m.VWAP.Update(candle)
	if err != nil {
		return err
	}

	if newSession {
		// Fetch and send new levels from completed sessions for tracking.
		levels, err := m.FetchNewLevels()
		if err != nil {
			return fmt.Errorf("fetching new levels: %w", err)
		}

		for idx := range levels {
			m.SendLevel(*levels[idx])
		}
	}

	return nil
}
