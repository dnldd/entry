package indicator

import (
	"fmt"
	"time"

	"github.com/dnldd/entry/shared"
	"go.uber.org/atomic"
)

const (
	// VwapReset is the vwap reset time (in new york time).
	VwapResetTime = "17:00:10"
)

// VWAP represents the Volume Weighted Average Price Indicator.
type VWAP struct {
	TypicalPriceVolume atomic.Float64
	Volume             atomic.Float64
	Market             string
	Timeframe          shared.Timeframe
	LastUpdateTime     atomic.Pointer[time.Time]
}

// NewVWAP initializes a VWAP for the provided market and timeframe.
func NewVWAP(market string, timeframe shared.Timeframe) *VWAP {
	return &VWAP{
		Market:    market,
		Timeframe: timeframe,
	}
}

// Update cummulatively updates the VWAP indicator with the provided candlestick data.
func (v *VWAP) Update(candle *shared.Candlestick) (float64, error) {
	if candle.Timeframe != v.Timeframe {
		return 0, fmt.Errorf("expected candles with timeframe %s, got %s",
			v.Timeframe.String(), candle.Timeframe.String())
	}

	typicalPrice := (candle.High + candle.Low + candle.Close) / 3
	v.TypicalPriceVolume.Add(typicalPrice * candle.Volume)
	v.Volume.Add(candle.Volume)

	if v.TypicalPriceVolume.Load() == 0 {
		return 0, nil
	}

	vwap := v.TypicalPriceVolume.Load() / v.Volume.Load()
	v.LastUpdateTime.Store(&candle.Date)
	candle.VWAP = vwap

	return vwap, nil
}

// Reset resets the VWAP indicator after a trading session.
func (v *VWAP) Reset() {
	v.TypicalPriceVolume.Store(0)
	v.Volume.Store(0)
}
