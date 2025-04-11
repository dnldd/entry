package shared

import (
	"math"
	"time"
)

const (
	// minimumVolumeSpikePercent is the minimum percentage difference in volume considered substantive.
	minimumVolumeSpikePercent = 0.35
)

// Momentum represents the momentum of a candlestick.
type Momentum int

const (
	High Momentum = iota
	Medium
	Low
)

// String stringifies the provided momentum.
func (m *Momentum) String() string {
	switch *m {
	case High:
		return "high"
	case Medium:
		return "medium"
	default:
		return "low"
	}
}

// Kind represents type of candlestick.
type Kind int

const (
	Marubozu Kind = iota
	Pinbar
	Doji
	Unknown
)

// String stringifies the candlestick kind.
func (k *Kind) String() string {
	switch *k {
	case Marubozu:
		return "marubozu"
	case Pinbar:
		return "pinbar"
	case Doji:
		return "doji"
	default:
		return "unknown"
	}
}

// Sentiment represents the candlestick sentiment.
type Sentiment int

const (
	Neutral Sentiment = iota
	Bullish
	Bearish
)

// String stringifies the provided sentiment.
func (s *Sentiment) String() string {
	switch *s {
	case Bullish:
		return "bullish"
	case Bearish:
		return "bearish"
	default:
		return "neutral"
	}
}

// Candlestick represents a unit candlestick for a market.
type Candlestick struct {
	Open   float64
	Low    float64
	High   float64
	Close  float64
	Volume float64
	Date   time.Time

	// Metadata and derived fields.
	Market    string
	Timeframe Timeframe
	VWAP      float64
}

// FetchSentiment returns the provided candlestick's sentiment.
func (c *Candlestick) FetchSentiment() Sentiment {
	sentiment := c.Close - c.Open
	switch {
	case sentiment < 0:
		return Bearish
	case sentiment > 0:
		return Bullish
	default:
		return Neutral
	}
}

// FetchKind returns the candlestick type.
func (c *Candlestick) FetchKind() Kind {
	if c.High == 0 || c.Low == 0 {
		return Unknown
	}

	candleRange := c.High - c.Low
	if candleRange == 0 {
		return Unknown
	}

	candleBody := math.Abs(c.Close - c.Open)
	upperWickRange := c.High - math.Max(c.Open, c.Close)
	lowerWickRange := math.Min(c.Open, c.Close) - c.Low

	bodyPercent := candleBody / candleRange
	upperWickPercent := upperWickRange / candleRange
	lowerWickPercent := lowerWickRange / candleRange

	switch {
	case (upperWickPercent >= 0.5 && lowerWickPercent <= 0.2) || (lowerWickPercent >= 0.5 && upperWickPercent <= 0.2):
		// If the candle body has one of its wicks being at least 50 percent of the candle, it's a pin bar.
		return Pinbar
	case bodyPercent <= 0.3 && upperWickPercent >= 0.3 && lowerWickPercent >= 0.3:
		// If the candle body is not more than 30 percent of the candle and has almost
		// identical wicks on both sides of it, it's a doji candle.
		return Doji
	case bodyPercent >= 0.7:
		// If the candle body accounts for over 70 percent of the candle, It is a marubozu candle.
		return Marubozu
	default:
		return Unknown
	}
}

// IsVolumeSpike checks whether there was a surge in volume for the current candle compared to
// the prevous candle.
func IsVolumeSpike(current *Candlestick, prev *Candlestick) bool {
	if prev.Volume == 0 || current.Volume == 0 {
		return false
	}

	diff := current.Volume - prev.Volume
	return diff > 0 && diff/prev.Volume >= minimumVolumeSpikePercent
}

// GenerateMomentum returns the current candles momentum.
func GenerateMomentum(current *Candlestick, prev *Candlestick) Momentum {
	if prev.Volume == 0 || current.Volume == 0 {
		return Low
	}

	switch {
	case IsVolumeSpike(current, prev):
		return High
	case current.Volume > prev.Volume:
		return Medium
	default:
		return Low
	}
}

// IsEngulfing detects whether the current candle engulfs the previous candle.
func IsEngulfing(current *Candlestick, prev *Candlestick) bool {
	currentKind := current.FetchKind()
	prevKind := prev.FetchKind()

	if currentKind == Doji || prevKind == Doji {
		// Exclude dojis from detecting engulfing candles.
		return false
	}

	// Detect bearish engulfing setups.
	isBearishEngulf := prev.Open < prev.Close && current.Open > current.Close &&
		current.Open >= prev.Close && current.Close <= prev.Open

	// Detect bullish engulfing setups.
	isBullishEngulf := prev.Open > prev.Close && current.Open < current.Close &&
		current.Open <= prev.Close && current.Close >= prev.Open

	if isBearishEngulf || isBullishEngulf {
		bodyPercent := math.Abs(current.Close-current.Open) / (current.High - current.Low)
		if bodyPercent < 0.5 {
			// Disqualify weaked bodied engulfing setups.
			return false
		}

		return true
	}

	return false
}

// CandleMetadata represents a candle's associated metadata.
type CandleMetadata struct {
	Kind      Kind
	Sentiment Sentiment
	Momentum  Momentum
	Volume    float64
	Engulfing bool
}
