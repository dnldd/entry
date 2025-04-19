package shared

// Reason represents an entry or exit reason.
type Reason int

const (
	TargetHit Reason = iota
	BullishEngulfing
	BearishEngulfing
	ReversalAtSupport
	ReversalAtResistance
	StrongVolume
	StrongMove
)

// String stringifies the provided reason.
func (r Reason) String() string {
	switch r {
	case TargetHit:
		return "target hit"
	case BullishEngulfing:
		return "bullish engulfing"
	case BearishEngulfing:
		return "bearish engulfing"
	case ReversalAtSupport:
		return "price reversal at support"
	case ReversalAtResistance:
		return "price reversal at resistance"
	case StrongVolume:
		return "strong volume"
	case StrongMove:
		return "strong move"
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
func (d Direction) String() string {
	switch d {
	case Long:
		return "long"
	case Short:
		return "short"
	default:
		return "unknown"
	}
}
