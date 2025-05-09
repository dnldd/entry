package shared

import (
	"fmt"
	"math"
	"time"

	"github.com/tidwall/gjson"
)

const (
	// MinimumVolumeSpikePercent is the minimum percentage difference in volume considered substantive.
	MinimumVolumeSpikePercent = 0.35
	// pointsRangeLimit is the number of points from entry reasonable for a stop loss.
	PointsRangeLimit = 12
	// MinimumPinbarLongestWickPercent is the minimum percentage the longest wick of a pinbar can be.
	MinimumPinbarLongestWickPercent = 0.5
	// MaximumDojiBodyPercent is the maximum body percentage for a doji.
	MaximumDojiBodyPercent = 0.3
	// MinimumDojiWickPercent is the minimum wick percent for a doji.
	MinimumDojiWickPercent = 0.3
	// MinimumMarubozuBodyPercent is the minimum body percentage for a marubozu.
	MinimumMarubozuBodyPercent = 0.7
)

// Momentum represents the momentum of a candlestick.
type Momentum int

const (
	High Momentum = iota
	Medium
	Low
)

// String stringifies the provided momentum.
func (m Momentum) String() string {
	switch m {
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
func (k Kind) String() string {
	switch k {
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
func (s Sentiment) String() string {
	switch s {
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
	Status    chan StatusCode
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
//
// Classifies the candle based on the closest match to the expected candle type
// not a perfect textbook definition.
func (c *Candlestick) FetchKind() Kind {
	if c.High == 0 || c.Low == 0 {
		return Unknown
	}

	if c.High <= c.Low {
		return Unknown
	}

	candleRange := c.High - c.Low
	candleBody := math.Abs(c.Close - c.Open)
	upperWickRange := c.High - math.Max(c.Open, c.Close)
	lowerWickRange := math.Min(c.Open, c.Close) - c.Low

	bodyPercent := candleBody / candleRange
	upperWickPercent := upperWickRange / candleRange
	lowerWickPercent := lowerWickRange / candleRange

	switch {
	case (upperWickPercent >= MinimumPinbarLongestWickPercent && upperWickPercent >= 2*lowerWickPercent) ||
		(lowerWickPercent >= MinimumPinbarLongestWickPercent && lowerWickPercent >= 2*upperWickPercent):
		// If the candle body has one of its wicks being at least 50 percent of the candle and it is
		// at least twice the length of the opposite wick, it's a pin bar.
		return Pinbar
	case bodyPercent <= MaximumDojiBodyPercent && upperWickPercent >= MinimumDojiWickPercent && lowerWickPercent >= MinimumDojiWickPercent:
		// If the candle body is not more than 30 percent of the candle and has almost
		// identical wicks on both sides of it, it's a doji candle.
		return Doji
	case bodyPercent >= MinimumMarubozuBodyPercent:
		// If the candle body accounts for over 70 percent of the candle, It is a marubozu candle.
		return Marubozu
	default:
		return Unknown
	}
}

// ParseCandlesticks parses candlesticks from the provided json data.
func ParseCandlesticks(data []gjson.Result, market string, timeframe Timeframe, loc *time.Location) ([]Candlestick, error) {
	candles := make([]Candlestick, len(data))

	for idx := range data {
		var candle Candlestick

		candle.Open = data[idx].Get("open").Float()
		candle.Low = data[idx].Get("low").Float()
		candle.High = data[idx].Get("high").Float()
		candle.Close = data[idx].Get("close").Float()
		candle.Volume = data[idx].Get("volume").Float()

		candle.Market = market
		candle.Timeframe = timeframe
		candle.Status = make(chan StatusCode, 1)

		dt, err := time.ParseInLocation(DateLayout, data[idx].Get("date").String(), loc)
		if err != nil {
			return nil, fmt.Errorf("parsing candlestick date: %w", err)
		}

		candle.Date = dt
		candles[idx] = candle
	}

	return candles, nil
}

// IsVolumeSpike checks whether there was a surge in volume for the current candle compared to
// the prevous candle.
func IsVolumeSpike(current *Candlestick, prev *Candlestick) bool {
	if current.Volume < 0 || prev.Volume < 0 || prev.Volume == 0 || current.Volume == 0 {
		return false
	}

	diff := current.Volume - prev.Volume
	return diff > 0 && diff/prev.Volume >= MinimumVolumeSpikePercent
}

// GenerateMomentum returns the current candles momentum.
func GenerateMomentum(current *Candlestick, prev *Candlestick) Momentum {
	if current.Volume < 0 || prev.Volume < 0 || prev.Volume == 0 || current.Volume == 0 {
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
			// Disqualify weak bodied engulfing setups.
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
	High      float64
	Low       float64
	Date      time.Time
}

// Strength returns the estimated strength of the provided candlestick.
func (m *CandleMetadata) Strength() uint32 {
	score := uint32(0)

	// Award scores bsased on candle structure tied with momentum.
	if m.Kind == Marubozu {
		switch m.Momentum {
		case High:
			score += 3
		case Medium:
			score += 2
		}
	}
	if m.Kind == Pinbar {
		switch m.Momentum {
		case High:
			score += 4
		case Medium:
			score += 3
		}
	}

	// An engulfing candle signifies strength.
	if m.Engulfing {
		score += 2
	}

	return score
}

// FetchSignalCandle returns the strongest candle from the provided set and sentiment.
func FetchSignalCandle(meta []*CandleMetadata, sentiment Sentiment) *CandleMetadata {
	var strongest *CandleMetadata
	var strongestStrength uint32

	for idx := range meta {
		current := meta[idx]
		if current.Sentiment != sentiment {
			// Signal candles must match sentiment.
			continue
		}

		if current.Kind == Doji {
			// Signal candles cannot be dojis.
			continue
		}

		currentStrength := current.Strength()
		if strongest == nil || currentStrength > strongestStrength ||
			(currentStrength == strongestStrength && current.Volume > strongest.Volume) {
			strongest = current
			strongestStrength = currentStrength
		}
	}

	return strongest
}

// CandleMetaRangeHighAndLow determines the high and low of the provided range of candle metadata.
func CandleMetaRangeHighAndLow(meta []*CandleMetadata) (float64, float64) {
	if len(meta) == 0 {
		return 0, 0
	}
	var high, low float64

	for idx := range meta {
		candleMeta := meta[idx]
		if high == 0 || candleMeta.High > high {
			high = candleMeta.High
		}

		if low == 0 || candleMeta.Low < low {
			low = candleMeta.Low
		}
	}

	return high, low
}

// AverageVolumeEntry represents an average volume entry.
type AverageVolumeEntry struct {
	Average   float64
	CreatedAt int64
}
