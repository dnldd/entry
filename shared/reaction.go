package shared

import "time"

// PriceMovement represents price movement relative to a point of interest.
type PriceMovement int

const (
	Above PriceMovement = iota
	Below
	Equal
)

// String stringifies the provided price movement.
func (m PriceMovement) String() string {
	switch m {
	case Above:
		return "above"
	case Below:
		return "below"
	case Equal:
		return "equal"
	default:
		return "unknown"
	}
}

// PriceReaction represents price reaction relative to a point of interest.
type PriceReaction int

const (
	Chop PriceReaction = iota
	Reversal
	Break
)

// String stringifies the provided reaction.
func (m PriceReaction) String() string {
	switch m {
	case Chop:
		return "chop"
	case Reversal:
		return "reversal"
	case Break:
		return "break"
	default:
		return "unknown"
	}
}
