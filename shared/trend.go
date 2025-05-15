package shared

import "math"

// Trend represents the market trend.
type Trend int

const (
	ChoppyTrend Trend = iota
	MildBullishTrend
	MildBearishTrend
	StrongBullishTrend
	StrongBearishTrend
	IntenseBullishTrend
	IntenseBearishTrend
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
	case IntenseBullishTrend:
		return "intense bullish trend"
	case IntenseBearishTrend:
		return "intense bearish trend"
	default:
		return "unknown trend"
	}
}

// CategorizeTrendScore classifies the provided trend score.
func CategorizeTrendScore(trendScore float64) Trend {
	// TODO: these thresholds have to be confirmed with tests.
	const minIntenseTrendThreshold = 0.01
	const minStrongTrendThreshold = 0.005
	const minMildTrendThreshold = 0.002

	positive := trendScore > 0

	trendScoreAbsolute := math.Abs(trendScore)
	switch {
	case trendScoreAbsolute > minIntenseTrendThreshold:
		if positive {
			return IntenseBullishTrend
		}
		return IntenseBearishTrend
	case trendScoreAbsolute > minStrongTrendThreshold && trendScoreAbsolute <= minIntenseTrendThreshold:
		if positive {
			return StrongBullishTrend
		}
		return StrongBearishTrend
	case trendScoreAbsolute > minMildTrendThreshold && trendScoreAbsolute <= minStrongTrendThreshold:
		if positive {
			return MildBullishTrend
		}
		return MildBearishTrend
	case trendScoreAbsolute < minMildTrendThreshold:
		return ChoppyTrend
	default:
		return ChoppyTrend
	}
}
