package shared

const (
	// PriceDataPayloadSize is the number of candles expected as payload for a price data range request.
	PriceDataPayloadSize = 4
	// MaxPriceDataRequestInterval is the maximum update intervals to wait before triggering a
	// price data request.
	MaxPriceDataRequestInterval = 3
)

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
