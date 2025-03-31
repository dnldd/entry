package main

import (
	"fmt"
	"time"
)

const (
	// Session names.
	asia    = "asia"
	london  = "london"
	newyork = "newyork"

	// Market session times in UTC without daylight savings.
	asiaOpen     = "22:00"
	asiaClose    = "06:00"
	londonOpen   = "06:00"
	londonClose  = "13:30"
	newYorkOpen  = "13:30"
	newYorkClose = "20:00"

	sessionTimeLayout = "15:04"
)

// SessionBounds represents the time bounds of a session.
type SessionBounds struct {
	Name  string
	Open  time.Time
	Close time.Time
}

// NewSessionBounds initializes new session bounds.
func NewSessionBounds(name string, open string, close string) (*SessionBounds, error) {
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

	bounds := &SessionBounds{
		Name:  name,
		Open:  sOpen,
		Close: sClose,
	}

	return bounds, nil
}

// IsCurrentSession checks whether the provided session is the current session.
func (s *SessionBounds) IsCurrentSession(now time.Time) bool {
	if now.After(s.Open) && now.Before(s.Close) {
		return true
	}

	return false
}

// SessionRange represents the price ranges of a session.
type SessionRange struct {
	Name string
	High float64
	Low  float64
}

// NewSessionRange instantiates a new session price range.
func NewSessionRange(name string) *SessionRange {
	return &SessionRange{
		Name: name,
	}
}

// UpdateRange updates the provided session's price range.
func (s *SessionRange) UpdateRange(currentPrice float64) {
	switch {
	case currentPrice < s.Low:
		s.Low = currentPrice
	case currentPrice > s.High:
		s.High = currentPrice
	}
}
