package shared

import "time"

// EntrySignal represents an entry signal for a position.
type EntrySignal struct {
	Market              string
	Timeframe           Timeframe
	Direction           Direction
	Price               float64
	Reasons             []Reason
	Confluence          uint32
	StopLoss            float64
	StopLossPointsRange float64
	CreatedOn           time.Time
	Done                chan struct{}
}

// ExitSignal represents an exit signal for a position.
type ExitSignal struct {
	Market     string
	Timeframe  Timeframe
	Direction  Direction
	Price      float64
	Reasons    []Reason
	Confluence uint32
	CreatedOn  time.Time
	Done       chan struct{}
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

// NewEntrySignal initializes a new entry signal.
func NewEntrySignal(market string, timeframe Timeframe, direction Direction, price float64,
	reasons []Reason, confluence uint32, created time.Time, stopLoss float64, stopLossPointsRange float64) EntrySignal {
	return EntrySignal{
		Market:              market,
		Timeframe:           timeframe,
		Direction:           direction,
		Price:               price,
		Reasons:             reasons,
		Confluence:          confluence,
		CreatedOn:           created,
		StopLoss:            stopLoss,
		StopLossPointsRange: stopLossPointsRange,
	}
}

// NewExitSignal initializes a new exit signal.
func NewExitSignal(market string, timeframe Timeframe, direction Direction, price float64,
	reasons []Reason, confluence uint32, created time.Time) ExitSignal {
	return ExitSignal{
		Market:     market,
		Timeframe:  timeframe,
		Direction:  direction,
		Price:      price,
		Reasons:    reasons,
		Confluence: confluence,
		CreatedOn:  created,
	}
}
