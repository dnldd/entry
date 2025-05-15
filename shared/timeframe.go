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
	// NewYorkLocation is the new york time location
	NewYorkLocation = "America/New_York"
)

// Timeframe represents the market data time period.
type Timeframe int

const (
	OneHour Timeframe = iota
	FiveMinute
	OneMinute
)

// String stringifies the provided timeframe.
func (t Timeframe) String() string {
	switch t {
	case OneHour:
		return "1H"
	case FiveMinute:
		return "5m"
	case OneMinute:
		return "1m"
	default:
		return "unknown"
	}
}

// NewYorkTime returns the current time in new york (EST/EDT adjusted automatically).
func NewYorkTime() (time.Time, *time.Location, error) {
	loc, err := time.LoadLocation(NewYorkLocation)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("loading new york timezone: %w", err)
	}

	now := time.Now().In(loc)
	return now, loc, nil
}

// NextInterval calculates the next expected time for the provided timeframe.
func NextInterval(timeframe Timeframe, currentTime time.Time) (time.Time, error) {
	switch timeframe {
	case FiveMinute:
		return currentTime.Truncate(time.Minute * 5).Add(time.Minute * 5), nil
	case OneHour:
		return currentTime.Truncate(time.Hour).Add(time.Hour), nil
	default:
		return time.Time{}, fmt.Errorf("unknown timeframe provided for interval: %s", timeframe.String())
	}
}
