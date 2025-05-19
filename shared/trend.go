package shared

// Trend represents the market trend.
type Trend int

const (
	ChoppyTrend Trend = iota
	MildBullishTrend
	MildBearishTrend
	StrongBullishTrend
	StrongBearishTrend
)

// String stringifies the provided trend.
func (t Trend) String() string {
	switch t {
	case ChoppyTrend:
		return "choppy trend"
	case MildBullishTrend:
		return "mild bullish trend"
	case MildBearishTrend:
		return "mild bearish trend"
	case StrongBullishTrend:
		return "strong bullish trend"
	case StrongBearishTrend:
		return "strong bearish trend"
	default:
		return "unknown trend"
	}
}
