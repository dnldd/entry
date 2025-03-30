package main

import (
	"context"
	"time"

	"github.com/tidwall/gjson"
)

// PositionStorer defines the requirements for storing positions.
type PositionStorer interface {
	// PersistClosedPosition stores the provided closed position to the database.
	PersistClosedPosition(ctx context.Context, position *Position) error
}

// MarketFetcher defines the requirements for fetching index market data.
type MarketFetcher interface {
	// FetchIndexIntradayHistorical fetches intraday historical market data.
	FetchIndexIntradayHistorical(ctx context.Context, market string, timeframe Timeframe, start time.Time, end time.Time) ([]gjson.Result, error)
}
