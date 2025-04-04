package main

import (
	"fmt"
	"time"
)

const (
	// Session names.
	asia    = "asia"
	london  = "london"
	newYork = "newyork"

	// Market session time in new york time (ET).
	asiaOpen     = "19:00"
	asiaClose    = "04:00"
	londonOpen   = "03:00"
	londonClose  = "12:00"
	newYorkOpen  = "08:00"
	newYorkClose = "17:00"

	sessionTimeLayout = "15:04"

	// maxSessions is the maximum number of sessions tracked by a market.
	maxSessions = 30
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
	sessionOpen, err := time.Parse(sessionTimeLayout, open)
	if err != nil {
		return nil, fmt.Errorf("parsing session open: %w", err)
	}

	sessionClose, err := time.Parse(sessionTimeLayout, close)
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

// Update updates the provided session's high and low, and whether they are ready to be used as levels.
func (s *Session) Update(candle *Candlestick) {
	switch {
	case candle.Low < s.Low:
		s.Low = candle.Low
	case candle.High > s.High:
		s.High = candle.High
	}
}

// IsCurrentSession checks whether the provided session is the current session.
func (s *Session) IsCurrentSession(current time.Time) bool {
	return (current.Equal(s.Open) || current.After(s.Open)) && current.Before(s.Close)
}
