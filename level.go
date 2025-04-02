package main

import "fmt"

// LevelKind represents the type of level.
type LevelKind int

const (
	Support LevelKind = iota
	Resistance
)

// String stringifies the provided level.
func (l *LevelKind) String() string {
	switch *l {
	case Support:
		return "support"
	case Resistance:
		return "resistance"
	default:
		return ""
	}
}

// Level represents a support or resistance level.
type Level struct {
	Price     float64
	Kind      LevelKind
	Reversals uint32
	Breaks    uint32
}

// NewLevel initializes a new level.
func NewLevel(price float64, kind LevelKind) *Level {
	return &Level{
		Price: price,
		Kind:  kind,
	}
}

// Update updates the provided level on whether there has been a price reversal or a level break.
func (l *Level) Update(priceReversal bool, levelBreak bool) error {
	if priceReversal {
		l.Reversals++
	}

	if levelBreak {
		l.Breaks++
		l.Reversals = 0

		switch l.Kind {
		case Support:
			l.Kind = Resistance
		case Resistance:
			l.Kind = Support
		default:
			return fmt.Errorf("unexpected level kind provided: %s", l.Kind.String())
		}
	}

	return nil
}
