package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tidwall/gjson"
)

const (
	dateFormat = "2006-01-02 15:04:05"
	base       = "https://financialmodelingprep.com/stable"
)

// Timeframe represents the market data time period.
type Timeframe int

const (
	OneHour Timeframe = iota
	FiveMinute
)

// String stringifies the provided timeframe.
func (t *Timeframe) String() string {
	switch *t {
	case OneHour:
		return "1H"
	case FiveMinute:
		return "5m"
	default:
		return ""
	}
}

// FMPConfig represents the configuration for the FMP client.
type FMPConfig struct {
	// APIkey is the FMP API Key.
	APIKey string
}

// FMPClient represents the Financial Modeling Preparation (FMP) API client.
type FMPClient struct {
	cfg   *FMPConfig
	httpc http.Client
	buf   *bytes.Buffer
}

// Ensure the FMPClient implements the MarketFetcher interface.
var _ MarketFetcher = (*FMPClient)(nil)

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
	c.buf.WriteString(base)
	c.buf.WriteString(path)
	c.buf.WriteString("?")
	c.buf.WriteString(params)
	url := c.buf.String()
	c.buf.Reset()

	return url
}

// ParseCandlesticks parses candlesticks from the provided json data.
func (c *FMPClient) ParseCandlesticks(data []gjson.Result, timeframe Timeframe) ([]Candlestick, error) {
	candles := make([]Candlestick, 0, len(data))

	for idx := range data {
		var candle Candlestick

		candle.Open = data[idx].Get("open").Float()
		candle.Low = data[idx].Get("low").Float()
		candle.High = data[idx].Get("high").Float()
		candle.Close = data[idx].Get("close").Float()
		candle.Volume = data[idx].Get("volume").Float()

		candle.Timeframe = timeframe

		dt, err := time.Parse(dateFormat, data[idx].Get("date").String())
		if err != nil {
			return nil, fmt.Errorf("parsing candlestick date: %w", err)
		}

		candle.Date = dt
		candles[idx] = candle
	}

	return candles, nil
}

// FetchIndexIntradayHistorical fetches intraday historical market data.
func (c *FMPClient) FetchIndexIntradayHistorical(ctx context.Context, market string, timeframe Timeframe, start time.Time, end time.Time) ([]gjson.Result, error) {
	const fiveMinuteHistoricalPath = "/historical-chart/5min"
	const oneHourHistoricalPath = "/historical-chart/1hour"

	params := url.Values{}
	params.Add("symbol", market)
	params.Add("apikey", c.cfg.APIKey)

	var formedURL string

	switch timeframe {
	case FiveMinute:
		formedURL = c.formURL(fiveMinuteHistoricalPath, params.Encode())
	case OneHour:
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

	data := gjson.GetBytes(body, "").Array()

	return data, nil
}
