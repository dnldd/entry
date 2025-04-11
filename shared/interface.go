package shared

import (
	"context"
	"time"

	"github.com/tidwall/gjson"
)

// EntryFinder defines the requirements for finding market entries.
type EntryFinder interface {
	// IsEntry analyzes the provided candle for entry conditions.
	IsEntry(ctx context.Context, candle *Candlestick) (bool, *EntrySignal)
}

// ExitFinder defines the requirements for finding market exits.
type ExitFinder interface {
	// IsExit analyzes the provided candle for exit conditions.
	IsExit(ctx context.Context, candle *Candlestick) (bool, *ExitSignal)
}

// MarketFetcher defines the requirements for fetching index market data.
type MarketFetcher interface {
	// FetchIndexIntradayHistorical fetches intraday historical market data.
	FetchIndexIntradayHistorical(ctx context.Context, market string, timeframe Timeframe, start time.Time, end time.Time) ([]gjson.Result, error)
}
