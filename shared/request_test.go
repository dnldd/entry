package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestRequestResponse(t *testing.T) {
	// Ensure requests can be created and can receive their responses on theit corresponding channels.
	market := "^GSPC"
	timeframe := FiveMinute
	candleMetaReq := NewCandleMetadataRequest(market, timeframe)
	assert.NotNil(t, candleMetaReq)
	go func() { candleMetaReq.Response <- []*CandleMetadata{} }()
	candleMetaResp := <-candleMetaReq.Response
	assert.Equal(t, candleMetaResp, []*CandleMetadata{})

	priceDataReq := NewPriceDataRequest(market, timeframe, 5)
	assert.NotNil(t, priceDataReq)
	go func() { priceDataReq.Response <- []*Candlestick{} }()
	priceDataResp := <-priceDataReq.Response
	assert.Equal(t, priceDataResp, []*Candlestick{})

	avgVolumeReq := NewAverageVolumeRequest(market, timeframe)
	assert.NotNil(t, avgVolumeReq)
	go func() { avgVolumeReq.Response <- float64(1) }()
	avgVolumeResp := <-avgVolumeReq.Response
	assert.Equal(t, avgVolumeResp, float64(1))

	marketSkewReq := NewMarketSkewRequest(market)
	assert.NotNil(t, marketSkewReq)
	go func() { marketSkewReq.Response <- LongSkewed }()
	marketSkewResp := <-marketSkewReq.Response
	assert.Equal(t, marketSkewResp, LongSkewed)

	now, _, _ := NewYorkTime()

	vwapReq := NewVWAPRequest(market, now, timeframe)
	assert.NotNil(t, vwapReq)
	go func() { vwapReq.Response <- &VWAP{Value: float64(3), Date: now} }()
	vwapResp := <-vwapReq.Response
	assert.Equal(t, vwapResp, &VWAP{Value: float64(3), Date: now})

	vwapDataReq := NewVWAPDataRequest(market, timeframe)
	assert.NotNil(t, vwapDataReq)
	go func() {
		vwapDataReq.Response <- []*VWAP{
			{Value: float64(3), Date: now},
			{Value: float64(4), Date: now.Add(time.Minute * 5)}}
	}()
	vwapDataResp := <-vwapDataReq.Response
	assert.Equal(t, vwapDataResp, []*VWAP{
		{Value: float64(3), Date: now},
		{Value: float64(4), Date: now.Add(time.Minute * 5)}})
}
