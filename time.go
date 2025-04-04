package main

import (
	"fmt"
	"time"
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
