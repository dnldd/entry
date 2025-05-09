package shared

import "time"

// VWAP represents a unit VWAP entry for a market.
type VWAP struct {
	Value float64
	Date  time.Time
}
