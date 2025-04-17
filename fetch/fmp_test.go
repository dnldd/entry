package fetch

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/tidwall/gjson"
)

// todo: mock the http client and return valid data.

func TestFMPClient(t *testing.T) {
	// Ensure the fmp client can be created.
	cfg := &FMPConfig{
		APIKey:  "key",
		BaseURL: "http://base",
	}

	fc := NewFMPClient(cfg)

	// Ensure urls can be formed accurately.
	params := url.Values{}
	params.Add("a", "bbb")
	params.Add("b", "ccc")

	path := "/path"
	formedUrl := fc.formURL(path, params.Encode())
	assert.Equal(t, formedUrl, "http://base/path?a=bbb&b=ccc")

	market := "^GSPC"
	timeframe := shared.FiveMinute
	data := `[{"open":10,"close":12,"high":15,"low":8, "volume":5,"date":"2025-02-04 15:05:00"}]`
	gjd := gjson.Parse(data).Array()

	// Ensure fetching historical candles can fail if the client is not configured properly.

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	threeMonthsAgo := now.AddDate(0, -3, 0)
	_, err = fc.FetchIndexIntradayHistorical(context.Background(), market, shared.FiveMinute, threeMonthsAgo, time.Time{})
	assert.Error(t, err)

	// Ensure candlesticks data can be parsed.
	candles, err := fc.ParseCandlesticks(gjd, market, timeframe)
	assert.NoError(t, err)
	assert.Equal(t, len(candles), 1)
	assert.Equal(t, candles[0].Open, float64(10))
	assert.Equal(t, candles[0].Close, float64(12))
	assert.Equal(t, candles[0].High, float64(15))
	assert.Equal(t, candles[0].Low, float64(8))
	assert.Equal(t, candles[0].Volume, float64(5))
	assert.Equal(t, candles[0].Date.Year(), 2025)
	assert.Equal(t, candles[0].Date.Month(), 2)
	assert.Equal(t, candles[0].Date.Day(), 4)
}
