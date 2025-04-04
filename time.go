package main

import (
	"fmt"
	"time"
)

const (
	sessionTimeLayout = "15:04"
	dateLayout        = "2006-01-02 15:04:05"
)

// NewYorkTime returns the current time in new york (EST/EDT adjusted automatically).
func NewYorkTime() (time.Time, *time.Location, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("loading new york timezone: %w", err)
	}

	now := time.Now().In(loc)
	return now, loc, nil
}
