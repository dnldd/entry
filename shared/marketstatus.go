package shared

// MarketStatus represents defines the possible market status states.
type MarketStatus int

const (
	NeutralInclination MarketStatus = iota
	LongInclined
	ShortInclined
)

// String stringifies the provided market status.
func (s MarketStatus) String() string {
	switch s {
	case NeutralInclination:
		return "neutral inclination"
	case LongInclined:
		return "long inclined"
	case ShortInclined:
		return "short inclined"
	default:
		return "unknown"
	}
}
