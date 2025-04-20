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

	// Market session time (futures) in new york time (ET).
	AsiaOpen     = "18:00"
	AsiaClose    = "03:00"
	LondonOpen   = "03:00"
	LondonClose  = "11:00"
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
func NewSession(name string, open string, close string, now time.Time) (*Session, error) {
	sessionOpen, err := time.Parse(SessionTimeLayout, open)
	if err != nil {
		return nil, fmt.Errorf("parsing session open: %w", err)
	}

	sessionClose, err := time.Parse(SessionTimeLayout, close)
	if err != nil {
		return nil, fmt.Errorf("parsing session close: %w", err)
	}

	loc := now.Location()
	if loc.String() != NewYorkLocation {
		return nil, fmt.Errorf("expected new york location for provided time, got %v", loc.String())
	}

	sOpen := time.Date(now.Year(), now.Month(), now.Day(), sessionOpen.Hour(), sessionOpen.Minute(), 0, 0, loc)
	sClose := time.Date(now.Year(), now.Month(), now.Day(), sessionClose.Hour(), sessionClose.Minute(), 0, 0, loc)
	if sClose.Before(sOpen) {
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

// CurrentSession returns the current active session name.
func CurrentSession(now time.Time) (string, error) {
	yesterday := now.AddDate(0, 0, -1)

	sessions := []struct {
		name  string
		open  string
		close string
		time  time.Time
	}{
		{Asia, AsiaOpen, AsiaClose, yesterday},
		{London, LondonOpen, LondonClose, now},
		{NewYork, NewYorkOpen, NewYorkClose, now},
		{Asia, AsiaOpen, AsiaClose, now},
	}

	for _, sess := range sessions {
		session, err := NewSession(sess.name, sess.open, sess.close, sess.time)
		if err != nil {
			return "", fmt.Errorf("creating %s session: %w", sess.name, err)
		}

		if (now.Equal(session.Open) || now.After(session.Open)) && now.Before(session.Close) {
			return session.Name, nil
		}
	}

	return "", nil
}

// IsMarketOpen checks whether the markets (only futures) are open by checking if the current
// time is within one of the market sessions.
func IsMarketOpen(now time.Time) (bool, string, error) {
	name, err := CurrentSession(now)
	if err != nil {
		return false, name, fmt.Errorf("fetching current market session: %v", err)
	}

	var open bool
	if name != "" {
		open = true
	}

	return open, name, nil
}
