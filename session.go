package main

import (
	"fmt"
	"slices"
	"time"
)

const (
	// Session names.
	asia    = "asia"
	london  = "london"
	newYork = "newyork"

	// Market session times in UTC without daylight savings.
	asiaOpen     = "22:00"
	asiaClose    = "06:00"
	londonOpen   = "06:00"
	londonClose  = "13:30"
	newYorkOpen  = "13:30"
	newYorkClose = "20:00"

	sessionTimeLayout = "15:04"

	// maxSessions is the maximum number of sessions tracked by a market.
	maxSessions = 30
)

// Session represents a market session.
type Session struct {
	Name    string
	High    float64
	Low     float64
	Open    time.Time
	Close   time.Time
	Matured bool
}

// NewSession initializes new market session.
func NewSession(name string, open string, close string) (*Session, error) {
	sessionOpen, err := time.Parse(sessionTimeLayout, open)
	if err != nil {
		return nil, fmt.Errorf("parsing session open: %w", err)
	}

	sessionClose, err := time.Parse(sessionTimeLayout, close)
	if err != nil {
		return nil, fmt.Errorf("parsing session close: %w", err)
	}

	now := time.Now().UTC()

	sOpen := time.Date(now.Year(), now.Month(), now.Day(), sessionOpen.Hour(), sessionOpen.Minute(), 0, 0, time.UTC)
	sClose := time.Date(now.Year(), now.Month(), now.Day(), sessionClose.Hour(), sessionClose.Minute(), 0, 0, time.UTC)
	if sClose.Before(sOpen) {
		// Adjust asia closes to the next day.
		sClose = sClose.Add(time.Hour * 24)
	}

	session := &Session{
		Name:  name,
		Open:  sOpen,
		Close: sClose,
	}

	return session, nil
}

// Update updates the provided session's high and low, and whether they are ready to be used as levels.
func (s *Session) Update(currentCandle Candlestick) {
	switch {
	case currentCandle.Low < s.Low:
		s.Low = currentCandle.Low
	case currentCandle.High > s.High:
		s.High = currentCandle.High
	}

	if !s.Matured {
		now := time.Now().UTC()

		// A sessions price range (high, low) is considered matured after an hour.
		if now.Sub(s.Open) > time.Hour {
			s.Matured = true
		}
	}
}

// IsCurrentSession checks whether the provided session is the current session.
func (s *Session) IsCurrentSession(now time.Time) bool {
	if (now.Equal(s.Open) || now.After(s.Open)) && now.Before(s.Close) {
		return true
	}

	return false
}

// Market tracks the metadata of a market.
type Market struct {
	Market   string
	Sessions []*Session
}

// NewMarket instantiates a new market.
func NewMarket(market string) *Market {
	return &Market{
		Market:   market,
		Sessions: make([]*Session, 0, maxSessions),
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
func (m *Market) Update(candle Candlestick) {
	now := time.Now().UTC()
	var currentSession *Session
	for idx := len(m.Sessions) - 1; idx > -1; idx-- {
		if m.Sessions[idx].IsCurrentSession(now) {
			currentSession = m.Sessions[idx]
			break
		}
	}

	currentSession.Update(candle)
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
