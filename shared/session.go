package shared

import (
	"fmt"
	"time"
)

const (
	// Session names.
	Asia    = "asia"
	London  = "london"
	NewYork = "newyork"

	// Market session time in new york time (ET).
	AsiaOpen     = "19:00"
	AsiaClose    = "04:00"
	LondonOpen   = "03:00"
	LondonClose  = "12:00"
	NewYorkOpen  = "08:00"
	NewYorkClose = "17:00"

	// maxSessions is the maximum number of sessions tracked by a market.
	maxSessions = 12

	// locality is the locale used for fetching time.
	locality = "America/New_York"
)

// Session represents a market session.
type Session struct {
	Name  string
	High  float64
	Low   float64
	Open  time.Time
	Close time.Time
}

// NewSession initializes new market session.
func NewSession(name string, open string, close string) (*Session, error) {
	sessionOpen, err := time.Parse(SessionTimeLayout, open)
	if err != nil {
		return nil, fmt.Errorf("parsing session open: %w", err)
	}

	sessionClose, err := time.Parse(SessionTimeLayout, close)
	if err != nil {
		return nil, fmt.Errorf("parsing session close: %w", err)
	}

	now, loc, err := NewYorkTime()
	if err != nil {
		return nil, err
	}

	sOpen := time.Date(now.Year(), now.Month(), now.Day(), sessionOpen.Hour(), sessionOpen.Minute(), 0, 0, loc)
	sClose := time.Date(now.Year(), now.Month(), now.Day(), sessionClose.Hour(), sessionClose.Minute(), 0, 0, loc)
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

// Update updates the provided session's high and low.
func (s *Session) Update(candle *Candlestick) {
	if s.Low == 0 {
		s.Low = candle.Low
	}
	if s.High == 0 {
		s.High = candle.High
	}
	if candle.Low < s.Low {
		s.Low = candle.Low
	}
	if candle.High > s.High {
		s.High = candle.High
	}
}

// IsCurrentSession checks whether the provided session is the current session.
func (s *Session) IsCurrentSession(current time.Time) bool {
	return (current.Equal(s.Open) || current.After(s.Open)) && current.Before(s.Close)
}

// IsMarketOpen checks whether the markets (only NQ currently) are open by checking if the current
// time is within one of the market sessions.
func IsMarketOpen(now time.Time) (bool, error) {
	if now.Location().String() != locality {
		return false, fmt.Errorf("time provided is not new york localized")
	}

	loc, err := time.LoadLocation(locality)
	if err != nil {
		return false, fmt.Errorf("loading new york timezone: %w", err)
	}

	sessions := []struct {
		Open  string
		Close string
	}{
		{AsiaOpen, AsiaClose},
		{LondonOpen, LondonClose},
		{NewYorkOpen, NewYorkClose},
	}

	var match bool
	for idx := range sessions {
		open, err := time.Parse(SessionTimeLayout, sessions[idx].Open)
		if err != nil {
			return false, fmt.Errorf("parsing open: %w", err)
		}
		close, err := time.Parse(SessionTimeLayout, sessions[idx].Close)
		if err != nil {
			return false, fmt.Errorf("parsing close: %w", err)
		}

		sOpen := time.Date(now.Year(), now.Month(), now.Day(), open.Hour(), open.Minute(), 0, 0, loc)
		sClose := time.Date(now.Year(), now.Month(), now.Day(), close.Hour(), close.Minute(), 0, 0, loc)
		if sClose.Before(sOpen) {
			// Adjust asia closes to the next day.
			sClose = sClose.Add(time.Hour * 24)
		}

		if (now.Equal(sOpen) || now.After(sOpen)) && now.Before(sClose) {
			match = true
		}

		if match {
			break
		}
	}

	return match, nil
}
