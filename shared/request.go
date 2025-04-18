package shared

// CandleMetadataRequest represents a request to fetch the current candle's metadata.
type CandleMetadataRequest struct {
	Market   string
	Response chan []*CandleMetadata
}

// PriceDataRequest represents a price data request to fetch price data for a time range.
type PriceDataRequest struct {
	Market   string
	Response chan []*Candlestick
}

// AverageVolumeRequest represents an average volume request to fetch the average
// volume for a market.
type AverageVolumeRequest struct {
	Market   string
	Response chan float64
}
