package priceaction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setupManager(t *testing.T, market string) *Manager {
	subs := make([]chan shared.Candlestick, 0, 10)
	subscribe := func(name string, sub chan shared.Candlestick) {
		subs = append(subs, sub)
	}

	requestPriceData := func(req shared.PriceDataRequest) {
		data := []*shared.Candlestick{}
		for idx := range 4 {
			candle := shared.Candlestick{
				Open:   float64(idx),
				Close:  float64(idx),
				High:   float64(idx),
				Low:    float64(idx),
				Volume: float64(idx),

				Market:    req.Market,
				Timeframe: shared.FiveMinute,
				Status:    make(chan shared.StatusCode, 1),
			}

			data = append(data, &candle)
		}

		go func() { req.Response <- data }()
	}
	levelReactionSignals := make(chan shared.ReactionAtLevel, 5)
	signalLevelReaction := func(reaction shared.ReactionAtLevel) {
		levelReactionSignals <- reaction
		reaction.Status <- shared.Processed
	}

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	vwapData := []*shared.VWAP{
		{
			Value: 4.2,
			Date:  now.Add(-time.Minute * 15),
		},
		{
			Value: 4.3,
			Date:  now.Add(-time.Minute * 10),
		},
		{
			Value: 4.4,
			Date:  now.Add(-time.Minute * 5),
		},
		{
			Value: 4.5,
			Date:  now,
		},
	}
	requestVWAPData := func(request shared.VWAPDataRequest) {
		request.Response <- vwapData
	}
	requestVWAP := func(request shared.VWAPRequest) {
		request.Response <- &shared.VWAP{
			Value: 4.5,
			Date:  now,
		}
	}
	vwapReactionSignals := make(chan shared.ReactionAtVWAP, 5)
	signalReactionAtVWAP := func(reaction shared.ReactionAtVWAP) {
		vwapReactionSignals <- reaction
		reaction.Status <- shared.Processed
	}

	imbalanceReactionSignals := make(chan shared.ReactionAtImbalance, 5)
	signalReactionAtImbalance := func(reaction shared.ReactionAtImbalance) {
		imbalanceReactionSignals <- reaction
		reaction.Status <- shared.Processed
	}

	cfg := &ManagerConfig{
		Markets:                   []string{market},
		Subscribe:                 subscribe,
		RequestPriceData:          requestPriceData,
		SignalReactionAtLevel:     signalLevelReaction,
		SignalReactionAtImbalance: signalReactionAtImbalance,
		SignalReactionAtVWAP:      signalReactionAtVWAP,
		RequestVWAPData:           requestVWAPData,
		RequestVWAP:               requestVWAP,
		FetchCaughtUpState: func(market string) (bool, error) {
			return true, nil
		},
		Logger: &log.Logger,
	}

	mgr, err := NewManager(cfg)
	assert.NoError(t, err)

	return mgr
}

func TestPriceActionManagerConfigValidate(t *testing.T) {
	// Dummy functions for required fields
	dummySubscribe := func(name string, sub chan shared.Candlestick) {}
	dummyRequestPriceData := func(request shared.PriceDataRequest) {}
	dummyRequestVWAPData := func(request shared.VWAPDataRequest) {}
	dummyRequestVWAP := func(request shared.VWAPRequest) {}
	dummySignalReactionAtLevel := func(signal shared.ReactionAtLevel) {}
	dummySignalReactionAtVWAP := func(signal shared.ReactionAtVWAP) {}
	dummySignalReactionAtImbalance := func(signal shared.ReactionAtImbalance) {}
	dummyFetchCaughtUpState := func(market string) (bool, error) { return true, nil }
	logger := zerolog.New(nil)

	baseCfg := &ManagerConfig{
		Markets:                   []string{"AAPL"},
		Subscribe:                 dummySubscribe,
		RequestPriceData:          dummyRequestPriceData,
		RequestVWAPData:           dummyRequestVWAPData,
		RequestVWAP:               dummyRequestVWAP,
		SignalReactionAtLevel:     dummySignalReactionAtLevel,
		SignalReactionAtVWAP:      dummySignalReactionAtVWAP,
		SignalReactionAtImbalance: dummySignalReactionAtImbalance,
		FetchCaughtUpState:        dummyFetchCaughtUpState,
		Logger:                    &logger,
	}

	tests := []struct {
		name        string
		modify      func(cfg *ManagerConfig)
		wantErr     bool
		errContains []string
	}{
		{
			name:    "valid config returns nil",
			modify:  func(cfg *ManagerConfig) { cfg.Logger = &logger },
			wantErr: false,
		},
		{
			name:        "missing Markets",
			modify:      func(cfg *ManagerConfig) { cfg.Markets = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"no markets provided"},
		},
		{
			name:        "missing Subscribe",
			modify:      func(cfg *ManagerConfig) { cfg.Subscribe = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"subscribe function cannot be nil"},
		},
		{
			name:        "missing RequestPriceData",
			modify:      func(cfg *ManagerConfig) { cfg.RequestPriceData = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"request price data function cannot be nil"},
		},
		{
			name:        "missing RequestVWAPData",
			modify:      func(cfg *ManagerConfig) { cfg.RequestVWAPData = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"request vwap data function cannot be nil"},
		},
		{
			name:        "missing SignalReactionAtLevel",
			modify:      func(cfg *ManagerConfig) { cfg.SignalReactionAtLevel = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"signal reaction at level function cannot be nil"},
		},
		{
			name:        "missing SignalReactionAtVWAP",
			modify:      func(cfg *ManagerConfig) { cfg.SignalReactionAtVWAP = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"signal reaction at vwap function cannot be nil"},
		},
		{
			name:        "missing SignalReactionAtImbalance",
			modify:      func(cfg *ManagerConfig) { cfg.SignalReactionAtImbalance = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"signal reaction at imbalance function cannot be nil"},
		},
		{
			name:        "missing FetchCaughtUpState",
			modify:      func(cfg *ManagerConfig) { cfg.FetchCaughtUpState = nil; cfg.Logger = &logger },
			wantErr:     true,
			errContains: []string{"fetch caught up state function cannot be nil"},
		},
		{
			name:        "missing Logger",
			modify:      func(cfg *ManagerConfig) { cfg.Logger = nil },
			wantErr:     true,
			errContains: []string{"logger cannot be nil"},
		},
		{
			name: "multiple missing fields",
			modify: func(cfg *ManagerConfig) {
				*cfg = ManagerConfig{}
			},
			wantErr: true,
			errContains: []string{
				"no markets provided",
				"subscribe function cannot be nil",
				"request price data function cannot be nil",
				"request vwap data function cannot be nil",
				"signal reaction at level function cannot be nil",
				"signal reaction at vwap function cannot be nil",
				"signal reaction at imbalance function cannot be nil",
				"fetch caught up state function cannot be nil",
				"logger cannot be nil",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := *baseCfg
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				for _, substr := range tt.errContains {
					assert.True(t, strings.Contains(err.Error(), substr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure the price action manager can be started.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Ensure the price action manager can receive market candlestick updates.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	mgr.SendMarketUpdate(firstCandle)

	// Ensure the price action manager can receive level signals.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  20,
		Status: make(chan shared.StatusCode, 1),
	}

	mgr.SendLevelSignal(levelSignal)

	// Ensure the price action manager can process candle metadata requests.
	candleMetaReq := shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata, 1),
	}

	mgr.SendCandleMetadataRequest(candleMetaReq)
	<-candleMetaReq.Response

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure the price action manage can process imbalance signals.
	imbalanceSignal := shared.ImbalanceSignal{
		Market: market,
		Imbalance: *shared.NewImbalance(market, shared.FiveMinute, float64(15), float64(10),
			float64(5), shared.Bullish, float64(0.5), now),
		Status: make(chan shared.StatusCode, 1),
	}

	mgr.SendImbalanceSignal(imbalanceSignal)
	<-imbalanceSignal.Status

	// Ensure the price action manager can be gracefully shutdown.
	cancel()
	<-done
}

func TestManagerHandleUpdateSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure handling update signals for an unknown market errors.
	wrongMarketCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    "^AAPL",
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleUpdateSignal(&wrongMarketCandle)
	assert.Error(t, err)

	// Ensure handling update signals for a valid market succeeds.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleUpdateSignal(&firstCandle)
	assert.NoError(t, err)

	// Add a level for reaction tests.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  3,
		Status: make(chan shared.StatusCode, 1),
	}
	err = mgr.handleLevelSignal(levelSignal)
	assert.NoError(t, err)

	// Trigger a price request for the market.
	mgr.markets[market].requestingPriceData.Store(true)

	// Ensure handling update signals when a price request is flagged processes the price request.
	secondCandle := shared.Candlestick{
		Open:   float64(10),
		Close:  float64(15),
		High:   float64(20),
		Low:    float64(9),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err = mgr.handleUpdateSignal(&secondCandle)
	assert.NoError(t, err)

	// Ensure price request flag is reset after the request is processed.
	assert.False(t, mgr.markets[market].requestingPriceData.Load())

	// Trigger a vwap and imbalance request for the market.
	mgr.markets[market].requestingVWAPData.Store(true)
	mgr.markets[market].requestingImbalanceData.Store(true)

	secondCandle.Status = make(chan shared.StatusCode, 1)
	err = mgr.handleUpdateSignal(&secondCandle)
	assert.NoError(t, err)

	// Ensure vwap request flag is reset after the request is processed.
	assert.False(t, mgr.markets[market].requestingVWAPData.Load())
}

func TestFillManagerChannels(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	candle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
	}

	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  5,
	}

	metaRequest := shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata),
	}

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	imbalanceSignal := shared.ImbalanceSignal{
		Market: market,
		Imbalance: *shared.NewImbalance(market, shared.FiveMinute, float64(15), float64(10),
			float64(5), shared.Bullish, float64(0.5), now),
		Status: make(chan shared.StatusCode, 1),
	}

	// Fill all the channels used by the manager.
	for range bufferSize + 1 {
		mgr.SendLevelSignal(levelSignal)
		mgr.SendMarketUpdate(candle)
		mgr.SendCandleMetadataRequest(metaRequest)
		mgr.SendImbalanceSignal(imbalanceSignal)
	}

	assert.Equal(t, len(mgr.metaSignals), bufferSize)
	assert.Equal(t, len(mgr.updateSignals), bufferSize)
	assert.Equal(t, len(mgr.levelSignals), bufferSize)
	assert.Equal(t, len(mgr.imbalanceSignals), bufferSize)
}

func TestManagerHandleLevelSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Add some market data.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleUpdateSignal(&firstCandle)
	assert.NoError(t, err)

	// Ensure handling level signals from an unknown market returns an error.
	wrongMarketLevelSignal := shared.LevelSignal{
		Market: "^AAPL",
		Price:  20,
		Status: make(chan shared.StatusCode, 1),
	}
	err = mgr.handleLevelSignal(wrongMarketLevelSignal)
	assert.Error(t, err)

	// Ensure handling level signals from a valid market is processed as expected.
	levelSignal := shared.LevelSignal{
		Market: market,
		Price:  20,
		Status: make(chan shared.StatusCode, 1),
	}
	err = mgr.handleLevelSignal(levelSignal)
	assert.NoError(t, err)
}

func TestManagerHandleCandleMetadataSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Ensure requesting candle metadate for an unknown market errors.
	req := shared.CandleMetadataRequest{
		Market:   "^AAPL",
		Response: make(chan []*shared.CandleMetadata),
	}

	err := mgr.handleCandleMetadataRequest(&req)
	assert.Error(t, err)

	// Generate candles to update the market.
	for idx := range 5 {
		candle := shared.Candlestick{
			Open:   float64(idx),
			Close:  float64(idx),
			High:   float64(idx),
			Low:    float64(idx),
			Volume: float64(idx),

			Market:    market,
			Timeframe: shared.FiveMinute,
			Status:    make(chan shared.StatusCode, 1),
		}

		err := mgr.handleUpdateSignal(&candle)
		assert.NoError(t, err)

	}

	// Ensure requesting candle metadata for a valid market succeeds.
	req = shared.CandleMetadataRequest{
		Market:   market,
		Response: make(chan []*shared.CandleMetadata),
	}

	go func() {
		<-req.Response
	}()

	err = mgr.handleCandleMetadataRequest(&req)
	assert.NoError(t, err)
}

func TestManagerHandleImbalanceSignal(t *testing.T) {
	// Ensure the price action manager can be created.
	market := "^GSPC"
	mgr := setupManager(t, market)

	// Add some market data.
	firstCandle := shared.Candlestick{
		Open:   float64(5),
		Close:  float64(8),
		High:   float64(9),
		Low:    float64(3),
		Volume: float64(2),

		Market:    market,
		Timeframe: shared.FiveMinute,
		Status:    make(chan shared.StatusCode, 1),
	}

	err := mgr.handleUpdateSignal(&firstCandle)
	assert.NoError(t, err)

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure handling level signals from an unknown market returns an error.
	wrongMarketImbalanceSignal := shared.ImbalanceSignal{
		Market: "^AAPL",
		Imbalance: *shared.NewImbalance(market, shared.FiveMinute, float64(15), float64(10),
			float64(5), shared.Bullish, float64(0.5), now),
		Status: make(chan shared.StatusCode, 1),
	}
	err = mgr.handleImbalanceSignal(wrongMarketImbalanceSignal)
	assert.Error(t, err)

	// Ensure handling imbalance signals from a valid market is processed as expected.
	imbalanceSignal := shared.ImbalanceSignal{
		Market: market,
		Imbalance: *shared.NewImbalance(market, shared.FiveMinute, float64(15), float64(10),
			float64(5), shared.Bullish, float64(0.5), now),
		Status: make(chan shared.StatusCode, 1),
	}
	err = mgr.handleImbalanceSignal(imbalanceSignal)
	assert.NoError(t, err)
}
