package shared

import "time"

const (
	// PriceDataPayloadSize is the number of candles expected as payload for a price data range request.
	PriceDataPayloadSize = 4
	// MaxPriceDataRequestInterval is the maximum update intervals to wait before triggering a
	// price data request.
	MaxPriceDataRequestInterval = 3
	// TimeoutDuration is the maximum time to wait before timing out.
	TimeoutDuration = time.Second * 4
)

// CandleMetadataRequest represents a request to fetch the current candle's metadata.
type CandleMetadataRequest struct {
	Market   string
	Response chan []*CandleMetadata
}

// NewCandleMetadataRequest initializes a new candle metadata request.
func NewCandleMetadataRequest(market string) *CandleMetadataRequest {
	return &CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*CandleMetadata, 1),
	}
}

// PriceDataRequest represents a price data request to fetch price data for a time range.
type PriceDataRequest struct {
	Market   string
	Response chan []*Candlestick
}

// NewPriceDataRequest initializes a new price data request.
func NewPriceDataRequest(market string) *PriceDataRequest {
	return &PriceDataRequest{
		Market:   market,
		Response: make(chan []*Candlestick, 1),
	}
}

// AverageVolumeRequest represents an average volume request to fetch the average
// volume for a market.
type AverageVolumeRequest struct {
	Market   string
	Response chan float64
}

// NewAverageVolumeRequest initializes a new average volume request.
func NewAverageVolumeRequest(market string) *AverageVolumeRequest {
	return &AverageVolumeRequest{
		Market:   market,
		Response: make(chan float64, 1),
	}
}

// MarketSkewRequest represents a market skew request to fetch the market
// skew for a market.
type MarketSkewRequest struct {
	Market   string
	Response chan MarketSkew
}

// NewMarketSkewRequest initializes a new market skew request.
func NewMarketSkewRequest(market string) *MarketSkewRequest {
	return &MarketSkewRequest{
		Market:   market,
		Response: make(chan MarketSkew, 1),
	}
}
