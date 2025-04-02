package main

// PriceMovement represents price movement relative to a point of interest.
type PriceMovement int

const (
	Close PriceMovement = iota
	Above
	Below
)

// String stringifies the provided price movement.
func (m *PriceMovement) String() string {
	switch *m {
	case Close:
		return "close"
	case Above:
		return "above"
	case Below:
		return "below"
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

// String stringifies the provided price movement.
func (m *PriceReaction) String() string {
	switch *m {
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
