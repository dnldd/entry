package shared

import (
	"go.uber.org/atomic"
)

// Imbalance represents market inefficiencies created by displacement. These act as high
// probability reaction levels for price.
type Imbalance struct {
	Market      string
	High        float64
	Midpoint    float64
	Low         float64
	Sentiment   Sentiment
	GapRatio    float64
	Purged      atomic.Bool
	Touches     atomic.Uint32
	Touching    atomic.Bool
	Invalidated atomic.Bool
}

// NewImbalance initializes a new imbalance.
func NewImbalance(market string, high float64, midpoint float64, low float64, sentiment Sentiment, gapRatio float64) *Imbalance {
	return &Imbalance{
		Market:    market,
		High:      high,
		Midpoint:  midpoint,
		Low:       low,
		Sentiment: sentiment,
		GapRatio:  gapRatio,
	}
}

// Apply updates the imbalance with the provided candstick.
func (imb *Imbalance) Apply(candle *Candlestick) {
	purged := imb.Purged.Load()
	invalidated := imb.Invalidated.Load()
	touching := imb.Touching.Load()

	if invalidated {
		return
	}

	switch imb.Sentiment {
	case Bullish:
		// If the imbalance is bullish then price closing below the low
		// of the imbalance range twice invalidates it.
		switch {
		case candle.Close < imb.Low && !purged:
			imb.Purged.Store(true)

		case candle.Close < imb.Low && purged:
			imb.Invalidated.Store(true)
		}

		// Price action within the imbalance high and low count as a single touch,
		// subsequent touches are only recorded when price resets by no longer touching
		// the imbalance.
		switch {
		case candle.Low <= imb.High && !touching:
			imb.Touching.Store(true)
			imb.Touches.Inc()
		case candle.Low > imb.High && touching:
			imb.Touching.Store(false)
		}

	case Bearish:
		// If the imbalance is bearish then price closing above the high
		// of the imbalance range twice invalidates it.
		switch {
		case candle.Close > imb.High && !purged:
			imb.Purged.Store(true)

		case candle.Close > imb.High && purged:
			imb.Invalidated.Store(true)
		}

		// Price action within the imbalance high and low count as a single touch,
		// subsequent touches are only recorded when price resets by no longer touching
		// the imbalance.
		switch {
		case candle.High >= imb.Low && !touching:
			imb.Touching.Store(true)
			imb.Touches.Inc()
		case candle.High < imb.Low && touching:
			imb.Touching.Store(false)
		}
	}
}

// Tagged determines whether price is having a fresh interaction with the imbalance zone.
func (imb *Imbalance) Tagged(candle *Candlestick) bool {
	invalidated := imb.Invalidated.Load()
	touching := imb.Touching.Load()

	if invalidated {
		return false
	}

	switch imb.Sentiment {
	case Bullish:
		if candle.Low <= imb.High && !touching {
			return true
		}
	case Bearish:
		if candle.High >= imb.Low && !touching {
			return true
		}
	}

	return false
}
