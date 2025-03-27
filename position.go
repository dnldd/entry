package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PositionStatus represents the status of a position.
type PositionStatus int

const (
	Active PositionStatus = iota
	StoppedOut
	Closed
)

// String stringifies the provided position status.
func (s *PositionStatus) String() string {
	switch *s {
	case Active:
		return "active"
	case StoppedOut:
		return "stopped out"
	case Closed:
		return "closed"
	default:
		return "unknown"
	}
}

// Direction represents market direction.
type Direction int

const (
	Long Direction = iota
	Short
)

// String stringifies the provided direction.
func (d *Direction) String() string {
	switch *d {
	case Long:
		return "long"
	case Short:
		return "short"
	default:
		return "unknown"
	}
}

// Position represents valid market position started by the given entry criteria.
type Position struct {
	ID            string
	Market        string
	Timeframe     string
	Direction     Direction
	StopLoss      float64
	PNLPercent    float64
	EntryPrice    float64
	EntryCriteria string
	ExitPrice     float64
	ExitCriteria  string
	Status        PositionStatus
	CreatedOn     uint64
	ClosedOn      uint64
}

// NewPosition initializes a new position.
func NewPosition(market string, timeframe string, direction Direction, entryPrice float64, entryCriteria string, stopLoss float64) *Position {
	return &Position{
		ID:            uuid.New().String(),
		Market:        market,
		Timeframe:     timeframe,
		Direction:     direction,
		CreatedOn:     uint64(time.Now().Unix()),
		EntryPrice:    entryPrice,
		EntryCriteria: entryCriteria,
		StopLoss:      stopLoss,
		Status:        Active,
	}
}

// ClosePosition closes the position using the provided exit details.
func (p *Position) ClosePosition(exitPrice float64, exitCriteria string) PositionStatus {
	p.ClosedOn = uint64(time.Now().Unix())
	p.ExitPrice = exitPrice
	p.ExitCriteria = exitCriteria

	switch {
	case p.ExitPrice > p.StopLoss && p.Direction == Short:
		p.Status = StoppedOut
	case p.ExitPrice < p.StopLoss && p.Direction == Long:
		p.Status = StoppedOut
	default:
		p.Status = Closed
	}

	return p.Status
}

// UpdatePNLPercent updates the percentage change of the position given the current price.
func (p *Position) UpdatePNLPercent(currentPrice float64) (float64, error) {
	switch {
	case p.Direction == Long:
		p.PNLPercent = ((currentPrice - p.EntryPrice) / p.EntryPrice) * 100
	case p.Direction == Short:
		p.PNLPercent = ((p.EntryPrice - currentPrice) / p.EntryPrice) * 100
	default:
		return 0, fmt.Errorf("unknown direction for position: %s", p.Direction.String())
	}

	return p.PNLPercent, nil
}
