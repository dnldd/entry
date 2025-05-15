package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/tidwall/gjson"
)

const (
	// BaseURL is the base url of the fmp service.
	BaseURL = "https://financialmodelingprep.com/stable"
)

// FMPConfig represents the configuration for the FMP client.
type FMPConfig struct {
	// APIkey is the FMP API Key.
	APIKey string
	// BaseURL is the base url of the FMP API.
	BaseURL string
}

// FMPClient represents the Financial Modeling Preparation (FMP) API client.
type FMPClient struct {
	cfg   *FMPConfig
	httpc http.Client
	buf   *bytes.Buffer
}

// Ensure the FMPClient implements the MarketFetcher interface.
var _ shared.MarketFetcher = (*FMPClient)(nil)

// NewFMPClient instantiates a new FMP client.
func NewFMPClient(cfg *FMPConfig) *FMPClient {
	return &FMPClient{
		cfg:   cfg,
		httpc: http.Client{Timeout: time.Second * 5},
		buf:   bytes.NewBuffer(make([]byte, 0, 512)),
	}
}

// formURL creates full urls including paramters for the api.
func (c *FMPClient) formURL(path string, params string) string {
	c.buf.WriteString(c.cfg.BaseURL)
	c.buf.WriteString(path)
	c.buf.WriteString("?")
	c.buf.WriteString(params)
	url := c.buf.String()
	c.buf.Reset()

	return url
}

// FetchIndexIntradayHistorical fetches intraday historical market data.
func (c *FMPClient) FetchIndexIntradayHistorical(ctx context.Context, market string, timeframe shared.Timeframe, start time.Time, end time.Time) ([]gjson.Result, error) {
	const oneHourHistoricalPath = "/historical-chart/1hour"
	const fiveMinuteHistoricalPath = "/historical-chart/5min"
	const oneMinuteHistoricalPath = "/historical-chart/1min"

	params := url.Values{}
	params.Add("symbol", market)
	params.Add("apikey", c.cfg.APIKey)
	params.Add("from", start.Format(shared.DateLayout))
	if !end.IsZero() {
		params.Add("to", end.Format(shared.DateLayout))
	}

	var formedURL string

	switch timeframe {
	case shared.OneMinute:
		formedURL = c.formURL(oneMinuteHistoricalPath, params.Encode())
	case shared.FiveMinute:
		formedURL = c.formURL(fiveMinuteHistoricalPath, params.Encode())
	case shared.OneHour:
		formedURL = c.formURL(oneHourHistoricalPath, params.Encode())
	default:
		return nil, fmt.Errorf("unknown timeframe provided: %s", timeframe.String())
	}

	resp, err := http.Get(formedURL)
	if err != nil {
		return nil, fmt.Errorf("fetching intraday historical data (%s) for %s: %w", timeframe.String(), market, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	data := gjson.ParseBytes(body).Array()

	return data, nil
}
