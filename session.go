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

// IsMarketOpen checks whether the markets (only NQ currently) are open by checking if the current
// time is within one of the market sessions.
func IsMarketOpen() (bool, error) {
	now, loc, err := NewYorkTime()
	if err != nil {
		return false, err
	}

	sessions := []struct {
		Open  string
		Close string
	}{
		{asiaOpen, asiaClose},
		{londonOpen, londonClose},
		{newYorkOpen, newYorkClose},
	}

	var match bool
	for idx := range sessions {
		open, err := time.Parse(sessionTimeLayout, sessions[idx].Open)
		if err != nil {
			return false, fmt.Errorf("parsing open: %w", err)
		}
		close, err := time.Parse(sessionTimeLayout, sessions[idx].Close)
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
