package shared

// MarketSkew represents defines the possible market skew states.
type MarketSkew int

const (
	NeutralSkew MarketSkew = iota
	LongSkewed
	ShortSkewed
)

// String stringifies the provided market skew.
func (s MarketSkew) String() string {
	switch s {
	case NeutralSkew:
		return "neutral skew"
	case LongSkewed:
		return "long skewed"
	case ShortSkewed:
		return "short skewed"
	default:
		return "unknown"
	}
}
