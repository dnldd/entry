package market

import (
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog/log"
)

func TestMarket(t *testing.T) {
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure a market can be created.
	levelSignals := make(chan shared.LevelSignal, 2)
	signalLevel := func(signal shared.LevelSignal) {
		levelSignals <- signal
		signal.Status <- shared.Processed
	}

	imbalanceSignals := make(chan shared.ImbalanceSignal, 2)
	signalImbalance := func(signal shared.ImbalanceSignal) {
		imbalanceSignals <- signal
		signal.Status <- shared.Processed
	}

	relayMarketUpdateSignals := make(chan shared.Candlestick, 20)
	relayMarketUpdate := func(candle shared.Candlestick) {
		candle.Status <- shared.Processed
		relayMarketUpdateSignals <- candle
	}

	market := "^GSPC"
	loc, err := time.LoadLocation(shared.NewYorkLocation)
	assert.NoError(t, err)

	cfg := &MarketConfig{
		Market:            market,
		Timeframes:        []shared.Timeframe{shared.OneMinute, shared.FiveMinute, shared.OneHour},
		SignalLevel:       signalLevel,
		SignalImbalance:   signalImbalance,
		RelayMarketUpdate: relayMarketUpdate,
		JobScheduler:      gocron.NewScheduler(loc),
		Logger:            &log.Logger,
	}

	asiaSessionCloseStr := "03:00"
	ts, err := time.Parse(shared.SessionTimeLayout, asiaSessionCloseStr)
	assert.NoError(t, err)
	asiaSessionCloseTime := time.Date(now.Year(), now.Month(), now.Day(), ts.Hour(), ts.Minute(), 0, 0, loc)

	mkt, err := NewMarket(cfg, asiaSessionCloseTime)
	assert.NoError(t, err)

	mkt.sessionSnapshot.GenerateNewSessions(asiaSessionCloseTime)

	// Ensure a market's caught up status can be set and fetched.
	mkt.SetCaughtUpStatus(true)
	status := mkt.CaughtUp()
	assert.Equal(t, status, true)

	// Ensure a market can be updated.
	firstCandle := &shared.Candlestick{
		Open:   float64(10),
		Close:  float64(9),
		High:   float64(11),
		Low:    float64(8),
		Volume: float64(2),
		Date:   asiaSessionCloseTime,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mkt.Update(firstCandle)
	assert.NoError(t, err)

	// Ensure a market can trigger session high/low signals.
	earlyNewYorkSessionTime := asiaSessionCloseTime.Add(time.Minute * 5)
	secondCandle := &shared.Candlestick{
		Open:   float64(9),
		Close:  float64(3),
		High:   float64(10),
		Low:    float64(2),
		Volume: float64(50),
		Date:   earlyNewYorkSessionTime,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mkt.Update(secondCandle)
	assert.NoError(t, err)

	levelHigh := <-levelSignals
	levelLow := <-levelSignals

	assert.Equal(t, levelHigh.Price, float64(11))
	assert.Equal(t, levelLow.Price, float64(8))

	// Ensure the market can generate imbalance signals.
	nextSessionTime := earlyNewYorkSessionTime.Add(time.Minute * 5)
	thirdCandle := &shared.Candlestick{
		Open:   float64(3),
		Close:  float64(2),
		High:   float64(4),
		Low:    float64(1),
		Volume: float64(3),
		Date:   nextSessionTime,

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mkt.Update(thirdCandle)
	assert.NoError(t, err)

	imb := <-imbalanceSignals

	assert.Equal(t, imb.Imbalance.Sentiment, shared.Bearish)
}
