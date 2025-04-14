package shared

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
func (r EntryReason) String() string {
	switch r {
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
func (r ExitReason) String() string {
	switch r {
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
