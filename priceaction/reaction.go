package priceaction

// Movement represents price movement relative to a point of interest.
type Movement int

const (
	Above Movement = iota
	Below
)

// String stringifies the provided price movement.
func (m *Movement) String() string {
	switch *m {
	case Above:
		return "above"
	case Below:
		return "below"
	default:
		return "unknown"
	}
}

// Reaction represents price reaction relative to a point of interest.
type Reaction int

const (
	Chop Reaction = iota
	Reversal
	Break
)

// String stringifies the provided price movement.
func (m *Reaction) String() string {
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
