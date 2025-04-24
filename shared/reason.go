package shared

// Reason represents an entry or exit reason.
type Reason int

const (
	TargetHit Reason = iota
	StopLossHit
	BullishEngulfing
	BearishEngulfing
	ReversalAtSupport
	ReversalAtResistance
	BreakBelowSupport
	BreakAboveResistance
	StrongVolume
	StrongMove
	HighVolumeSession
)

// String stringifies the provided reason.
func (r Reason) String() string {
	switch r {
	case TargetHit:
		return "target hit"
	case StopLossHit:
		return "stop loss hit"
	case BullishEngulfing:
		return "bullish engulfing"
	case BearishEngulfing:
		return "bearish engulfing"
	case ReversalAtSupport:
		return "price reversal at support"
	case ReversalAtResistance:
		return "price reversal at resistance"
	case BreakBelowSupport:
		return "price break below support"
	case BreakAboveResistance:
		return "price break above resistance"
	case StrongVolume:
		return "strong volume"
	case StrongMove:
		return "strong move"
	case HighVolumeSession:
		return "high volume session"
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
