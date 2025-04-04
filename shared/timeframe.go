package shared

import (
	"fmt"
	"time"
)

const (
	// SessionTimeLayout is the format layout for parsing session times in a day.
	SessionTimeLayout = "15:04"
	// DateLatout is the format layout for parsing dates.
	DateLayout = "2006-01-02 15:04:05"
)

// Timeframe represents the market data time period.
type Timeframe int

const (
	OneHour Timeframe = iota
	FiveMinute
)

// String stringifies the provided timeframe.
func (t *Timeframe) String() string {
	switch *t {
	case OneHour:
		return "1H"
	case FiveMinute:
		return "5m"
	default:
		return "unknown"
	}
}

// NewYorkTime returns the current time in new york (EST/EDT adjusted automatically).
func NewYorkTime() (time.Time, *time.Location, error) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("loading new york timezone: %w", err)
	}

	now := time.Now().In(loc)
	return now, loc, nil
}
