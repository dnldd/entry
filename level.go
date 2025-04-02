package main

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
		return "unknown"
	}
}

// Level represents a support or resistance level.
type Level struct {
	Market    string
	Price     float64
	Kind      LevelKind
	Reversals uint32
	Breaks    uint32
}

// NewLevel initializes a new level.
func NewLevel(market string, price float64) *Level {
	return &Level{
		Market: market,
		Price:  price,
	}
}

// Update updates the provided level based on the provided price reaction.
func (l *Level) Update(reaction PriceReaction) {
	switch reaction {
	case Chop:
		// do nothing.
	case Reversal:
		l.Reversals++
	case Break:
		l.Breaks++
		l.Reversals = 0

		switch l.Kind {
		case Support:
			l.Kind = Resistance
		default:
			l.Kind = Support
		}
	}
}
