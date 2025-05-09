package indicator

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestVWAPGenerator(t *testing.T) {
	// Ensure vwap can be created.
	market := "^GSPC"
	timeframe := shared.FiveMinute
	vwap := NewVWAP(market, timeframe)

	// Ensure vwap generator ignores update candles that are not of the expected timeframe.
	ignoredCandle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.OneHour,
	}

	_, err := vwap.Update(ignoredCandle)
	assert.Error(t, err)

	// Ensure vwap can be zero.
	candle := &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(0),

		Market:    market,
		Timeframe: timeframe,
	}

	vwp, err := vwap.Update(candle)
	assert.NoError(t, err)
	assert.Equal(t, vwp.Value, 0)

	// Ensure vwap can be updated.
	candle = &shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: timeframe,
	}

	vwp, err = vwap.Update(candle)
	assert.NoError(t, err)
	assert.GreaterThan(t, vwp.Value, 0)
	assert.GreaterThan(t, vwap.TypicalPriceVolume.Load(), 0)
	assert.GreaterThan(t, vwap.Volume.Load(), 0)

	// Ensure vwap indicator can be reset.
	vwap.Reset()
	assert.Equal(t, vwap.Volume.Load(), 0)
	assert.Equal(t, vwap.TypicalPriceVolume.Load(), 0)
}
