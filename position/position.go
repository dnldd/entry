package position

import (
	"bytes"
	"fmt"

	"github.com/dnldd/entry/shared"
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

// Position represents valid market position started by the given entry criteria.
type Position struct {
	ID           string
	Market       string
	Timeframe    shared.Timeframe
	Direction    shared.Direction
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
func stringifyEntryReasons(reasons []shared.EntryReason) string {
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
func stringifyExitReasons(reasons []shared.ExitReason) string {
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
func NewPosition(entry *shared.EntrySignal) (*Position, error) {
	if entry == nil {
		return nil, fmt.Errorf("entry signal cannot be nil")
	}

	now, _, err := shared.NewYorkTime()
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
func (p *Position) ClosePosition(exit *shared.ExitSignal) (PositionStatus, error) {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return Closed, err
	}

	p.ClosedOn = uint64(now.Unix())
	p.ExitPrice = exit.Price
	p.ExitReasons = stringifyExitReasons(exit.Reasons)

	switch {
	case p.ExitPrice > p.StopLoss && p.Direction == shared.Short:
		p.Status = StoppedOut
	case p.ExitPrice < p.StopLoss && p.Direction == shared.Long:
		p.Status = StoppedOut
	default:
		p.Status = Closed
	}

	return p.Status, nil
}

// UpdatePNLPercent updates the percentage change of the position given the current price.
func (p *Position) UpdatePNLPercent(currentPrice float64) (float64, error) {
	switch {
	case p.Direction == shared.Long:
		p.PNLPercent = ((currentPrice - p.EntryPrice) / p.EntryPrice) * 100
	case p.Direction == shared.Short:
		p.PNLPercent = ((p.EntryPrice - currentPrice) / p.EntryPrice) * 100
	default:
		return 0, fmt.Errorf("unknown direction for position: %s", p.Direction.String())
	}

	return p.PNLPercent, nil
}
