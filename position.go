package main

import (
	"bytes"
	"fmt"

	"github.com/google/uuid"
)

// EntryReason represents an entry reason.
type EntryReason int

const (
	BullishEngulfingEntry EntryReason = iota
	BearishEngulfingEntry
	ReversalAtSupportEntry
	ReversalAtResistanceEntry
	StrongVolumeEntry
)

// String stringifies the provided entry reason.
func (r *EntryReason) String() string {
	switch *r {
	case BullishEngulfingEntry:
		return "bullish engulfing"
	case BearishEngulfingEntry:
		return "bearish engulfing"
	case ReversalAtSupportEntry:
		return "price reversal at support"
	case ReversalAtResistanceEntry:
		return "price reversal at resistance"
	case StrongVolumeEntry:
		return "strong volume"
	default:
		return "unknown"
	}
}

// ExitReason represents an exit reason.
type ExitReason int

const (
	TargetHitExit ExitReason = iota
	BullishEngulfingExit
	BearishEngulfingExit
	ReversalAtSupportExit
	ReversalAtResistanceExit
	StrongVolumeExit
)

// String stringifies the provided exit reason.
func (r *ExitReason) String() string {
	switch *r {
	case TargetHitExit:
		return "target hit"
	case BullishEngulfingExit:
		return "bullish engulfing"
	case BearishEngulfingExit:
		return "bearish engulfing"
	case ReversalAtSupportExit:
		return "price reversal at support"
	case ReversalAtResistanceExit:
		return "price reversal at resistance"
	case StrongVolumeExit:
		return "strong volume"
	default:
		return "unknown"
	}
}

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
	ID           string
	Market       string
	Timeframe    Timeframe
	Direction    Direction
	StopLoss     float64
	PNLPercent   float64
	EntryPrice   float64
	EntryReasons string
	ExitPrice    float64
	ExitReasons  string
	Status       PositionStatus
	CreatedOn    uint64
	ClosedOn     uint64
}

// stringifyEntryReasons stringifies the collection of entry reasons provided.
func stringifyEntryReasons(reasons []EntryReason) string {
	buf := bytes.NewBuffer([]byte{})
	for idx := range reasons {
		buf.WriteString(reasons[idx].String())
		if idx < len(reasons)-1 {
			buf.WriteString(",")
		}
	}

	return buf.String()
}

// stringifyExitReasons stringifies the collection of exit reasons provided.
func stringifyExitReasons(reasons []ExitReason) string {
	buf := bytes.NewBuffer([]byte{})
	for idx := range reasons {
		buf.WriteString(reasons[idx].String())
		if idx < len(reasons)-1 {
			buf.WriteString(",")
		}
	}

	return buf.String()
}

// NewPosition initializes a new position.
func NewPosition(entry *EntrySignal) (*Position, error) {
	now, _, err := NewYorkTime()
	if err != nil {
		return nil, err
	}

	pos := &Position{
		ID:           uuid.New().String(),
		Market:       entry.Market,
		Timeframe:    entry.Timeframe,
		Direction:    entry.Direction,
		CreatedOn:    uint64(now.Unix()),
		EntryPrice:   entry.Price,
		EntryReasons: stringifyEntryReasons(entry.Reasons),
		StopLoss:     entry.StopLoss,
		Status:       Active,
	}

	return pos, nil
}

// ClosePosition closes the position using the provided exit details.
func (p *Position) ClosePosition(exit *ExitSignal) (PositionStatus, error) {
	now, _, err := NewYorkTime()
	if err != nil {
		return Closed, err
	}

	p.ClosedOn = uint64(now.Unix())
	p.ExitPrice = exit.Price
	p.ExitReasons = stringifyExitReasons(exit.Reasons)

	switch {
	case p.ExitPrice > p.StopLoss && p.Direction == Short:
		p.Status = StoppedOut
	case p.ExitPrice < p.StopLoss && p.Direction == Long:
		p.Status = StoppedOut
	default:
		p.Status = Closed
	}

	return p.Status, nil
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
