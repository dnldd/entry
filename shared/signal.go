package shared

import (
	"time"
)

// StatusCode represents a request or signal status code.
type StatusCode int

const (
	Processing StatusCode = iota
	Processed
)

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
	Status              chan StatusCode
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
		Status:              make(chan StatusCode, 1),
	}
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
	Status     chan StatusCode
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
		Status:     make(chan StatusCode, 1),
	}
}

// LevelSignal represents a level signal to outline a price level.
type LevelSignal struct {
	Market string
	Price  float64
	Close  float64
	Status chan StatusCode
}

// NewLevelSignal initializes a new level signal.
func NewLevelSignal(market string, price float64, close float64) LevelSignal {
	return LevelSignal{
		Market: market,
		Price:  price,
		Close:  close,
		Status: make(chan StatusCode, 1),
	}
}

// CatchUpSignal represents a signal to catchup on market data.
type CatchUpSignal struct {
	Market    string
	Timeframe []Timeframe
	Start     time.Time
	Status    chan StatusCode
}

// NewCatchUpSignal initializes a new catch up signal.
func NewCatchUpSignal(market string, timeframe []Timeframe, start time.Time) CatchUpSignal {
	return CatchUpSignal{
		Market:    market,
		Timeframe: timeframe,
		Start:     start,
		Status:    make(chan StatusCode, 1),
	}
}

// CaughtUpSignal represents a signal to conclude a catch up on market data.
type CaughtUpSignal struct {
	Market string
	Status chan StatusCode
}

// NewCaughtUpSignal initializes a new caught up signal.
func NewCaughtUpSignal(market string) CaughtUpSignal {
	return CaughtUpSignal{
		Market: market,
		Status: make(chan StatusCode, 1),
	}
}
