package candlestick

import (
	"time"

	"github.com/dnldd/entry/shared"
)

// Candlestick represents a unit candlestick for a market.
type Candlestick struct {
	Open   float64
	Low    float64
	High   float64
	Close  float64
	Volume float64
	Date   time.Time

	// Metadata and derived fields.
	Market    string
	Timeframe shared.Timeframe
	VWAP      float64
}
