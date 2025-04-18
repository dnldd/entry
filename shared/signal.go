package shared

import "time"

// EntrySignal represents an entry signal for a position.
type EntrySignal struct {
	Market    string
	Timeframe Timeframe
	Direction Direction
	Price     float64
	Reasons   []Reason
	StopLoss  float64
	Done      chan struct{}
}

// ExitSignal represents an exit signal for a position.
type ExitSignal struct {
	Market    string
	Timeframe Timeframe
	Direction Direction
	Price     float64
	Reasons   []Reason
	Done      chan struct{}
}

// LevelSignal represents a level signal to outline a price level.
type LevelSignal struct {
	Market string
	Price  float64
	Done   chan struct{}
}

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe Timeframe
	Start     time.Time
	Done      chan struct{}
}

// CaughtUpSignal represents a signal to conclude a catch up on market data.
type CaughtUpSignal struct {
	Market string
	Done   chan struct{}
}
