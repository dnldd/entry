package shared

import (
	"context"
	"time"

	"github.com/tidwall/gjson"
)

// MarketFetcher defines the requirements for fetching index market data.
type MarketFetcher interface {
	// FetchIndexIntradayHistorical fetches intraday historical market data.
	FetchIndexIntradayHistorical(ctx context.Context, market string, timeframe Timeframe, start time.Time, end time.Time) ([]gjson.Result, error)
}
