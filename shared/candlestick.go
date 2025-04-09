package shared

import (
	"math"
	"time"
)

const (
	// minimumVolumeDifferencePercent is the minimum difference in volume considered substantive.
	minimumVolumeDifferencePercent = 0.2
)

// Momentum represents the momentum of a candlestick.
type Momentum int

const (
	High Momentum = iota
	Medium
	Low
)

// Kind represents type of candlestick.
type Kind int

const (
	Marubozu Kind = iota
	Pinbar
	Doji
	Unknown
)

// Sentiment represents the candlestick sentiment.
type Sentiment int

const (
	Neutral Sentiment = iota
	Bullish
	Bearish
)

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
	case bodyPercent <= 0.3 && (upperWickPercent >= 0.6 || lowerWickPercent >= 0.6):
		// If the candle body is not more than 30 percent of the candle and has one of its wicks
		// being at least 60 percent of the candle, it's a pin bar.
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

// FetchMomentum returns the current candles momentum.
func FetchMomentum(current *Candlestick, prev *Candlestick) Momentum {
	if prev.Volume == 0 {
		return Low
	}

	volumeDifference := current.Volume - prev.Volume
	volumeDifferencePercent := volumeDifference / prev.Volume

	kind := current.FetchKind()
	switch {
	case kind == Marubozu:
		switch {
		case volumeDifference > 0 && volumeDifferencePercent >= minimumVolumeDifferencePercent:
			return High
		case volumeDifference > 0 && volumeDifferencePercent < minimumVolumeDifferencePercent:
			return Medium
		default:
			// If there is a marubozu candle with little to no volume backing it, it is likely a
			// momentum trap. Avoid it.
			return Low
		}
	default:
		return Low
	}
}
