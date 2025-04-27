package fetch

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
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

	// Ensure fetching historical candles can fail if the client is not configured properly.

	market := "^GSPC"
	timeframe := shared.FiveMinute
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	threeMonthsAgo := now.AddDate(0, -3, 0)
	_, err = fc.FetchIndexIntradayHistorical(context.Background(), market, timeframe, threeMonthsAgo, time.Time{})
	assert.Error(t, err)
}
