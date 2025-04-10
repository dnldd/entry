package shared

// CandleMetadataRequest represents a request to fetch the current candle's metadata.
type CandleMetadataRequest struct {
	Market   string
	Response *chan CandleMetadata
}

// PriceDataRequest represents a price data request to fetch price data for a time range.
type PriceDataRequest struct {
	Market   string
	Response *chan []*Candlestick
}
