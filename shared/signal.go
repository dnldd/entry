package shared

import "time"

// EntrySignal represents an entry signal for a position.
type EntrySignal struct {
	Market    string
	Timeframe Timeframe
	Direction Direction
	Price     float64
	Reasons   []EntryReason
	StopLoss  float64
}

// ExitSignal represents an exit signal for a position.
type ExitSignal struct {
	Market    string
	Timeframe Timeframe
	Direction Direction
	Price     float64
	Reasons   []ExitReason
}

// LevelSignal represents a level signal to outline a price level.
type LevelSignal struct {
	Market string
	Price  float64
}

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe Timeframe
	Start     time.Time
}

// CatchUpCompleteSignal represents concluding signal on market data catch up process.
type CatchUpCompleteSignal struct {
	Market string
}
