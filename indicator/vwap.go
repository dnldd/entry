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

// VWAP represents a unit VWAP entry for a market.
type VWAP struct {
	Value float64
	Date  time.Time
}

// VWAPGenerator represents the Volume Weighted Average Price Indicator.
type VWAPGenerator struct {
	TypicalPriceVolume atomic.Float64
	Volume             atomic.Float64
	Current            atomic.Pointer[VWAP]
	Market             string
	Timeframe          shared.Timeframe
	LastUpdateTime     atomic.Pointer[time.Time]
}

// NewVWAPGenerator initializes a VWAP indicator for the provided market and timeframe.
func NewVWAPGenerator(market string, timeframe shared.Timeframe) *VWAPGenerator {
	return &VWAPGenerator{
		Market:    market,
		Timeframe: timeframe,
	}
}

// Update cummulatively updates the VWAP indicator with the provided candlestick data.
func (v *VWAPGenerator) Update(candle *shared.Candlestick) (*VWAP, error) {
	if candle.Timeframe != v.Timeframe {
		return nil, fmt.Errorf("expected candles with timeframe %s, got %s",
			v.Timeframe.String(), candle.Timeframe.String())
	}

	typicalPrice := (candle.High + candle.Low + candle.Close) / 3
	v.TypicalPriceVolume.Add(typicalPrice * candle.Volume)
	v.Volume.Add(candle.Volume)

	vwap := &VWAP{
		Date: candle.Date,
	}

	if v.TypicalPriceVolume.Load() == 0 {
		return vwap, nil
	}

	val := v.TypicalPriceVolume.Load() / v.Volume.Load()
	vwap.Value = val
	v.Current.Store(vwap)
	v.LastUpdateTime.Store(&candle.Date)

	return vwap, nil
}

// Reset resets the VWAP indicator after a trading session.
func (v *VWAPGenerator) Reset() {
	v.TypicalPriceVolume.Store(0)
	v.Volume.Store(0)
}
