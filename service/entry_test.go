package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/dnldd/entry/engine"
	"github.com/dnldd/entry/fetch"
	"github.com/dnldd/entry/market"
	"github.com/dnldd/entry/position"
	"github.com/dnldd/entry/priceaction"
	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

func setupEntry() (*Entry, chan shared.LevelSignal, chan shared.LevelReaction, chan shared.EntrySignal, error) {
	var err error
	var marketMgr *market.Manager
	var fetchMgr *fetch.Manager
	var positionMgr *position.Manager
	var priceActionMgr *priceaction.Manager
	var historicData *shared.HistoricData
	var entryEngine *engine.Engine

	mkt := "^GSPC"
	cfg := &EntryConfig{
		Markets:              []string{mkt},
		FMPAPIKey:            "key",
		Backtest:             true,
		BacktestMarket:       mkt,
		BacktestDataFilepath: "../testdata/historicdata.json",
	}

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	logger := log.With().Str("service", "entry").Logger()

	caughtUpFunc := func(signal shared.CaughtUpSignal) {
		if marketMgr != nil {
			marketMgr.SendCaughtUpSignal(signal)
		}
	}

	notifySubcribersFunc := func(candle shared.Candlestick) error {
		if fetchMgr != nil {
			return fetchMgr.NotifySubscribers(candle)
		}

		return nil
	}

	now, loc, err := shared.NewYorkTime()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fetching new york time: %v", err)
	}

	if cfg.Backtest {
		// Ensure the service starts at the time denoted by the historical data
		// supplied for backtests.
		historicDataLogger := logger.With().Str("component", "historicdata").Logger()
		historicData, err = shared.NewHistoricData(&shared.HistoricDataConfig{
			Market:            cfg.BacktestMarket,
			Timeframe:         shared.FiveMinute,
			FilePath:          cfg.BacktestDataFilepath,
			SignalCaughtUp:    caughtUpFunc,
			NotifySubscribers: notifySubcribersFunc,
			Logger:            &historicDataLogger,
		})
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("creating historic data: %v", err)
		}

		now = historicData.FetchStartTime()
	}

	jobScheduler := gocron.NewScheduler(loc)

	fmp := fetch.NewFMPClient(&fetch.FMPConfig{
		APIKey:  cfg.FMPAPIKey,
		BaseURL: fetch.BaseURL,
	})

	fetchMgrLogger := logger.With().Str("component", "fetchmanager").Logger()
	fetchMgr, err = fetch.NewManager(&fetch.ManagerConfig{
		Markets:        cfg.Markets,
		ExchangeClient: fmp,
		SignalCaughtUp: caughtUpFunc,
		JobScheduler:   jobScheduler,
		Logger:         &fetchMgrLogger,
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("creating fetch manager: %v", err)
	}

	signalLevelLeak := make(chan shared.LevelSignal, 10)
	signalLevelFunc := func(signal shared.LevelSignal) {
		if priceActionMgr != nil {
			signalLevelLeak <- signal
			priceActionMgr.SendLevelSignal(signal)
		}
	}

	marketMgrLogger := logger.With().Str("component", "marketmanager").Logger()
	marketMgr, err = market.NewManager(&market.ManagerConfig{
		Markets:      cfg.Markets,
		Backtest:     cfg.Backtest,
		Subscribe:    fetchMgr.Subscribe,
		CatchUp:      fetchMgr.SendCatchUpSignal,
		SignalLevel:  signalLevelFunc,
		JobScheduler: jobScheduler,
		Logger:       &marketMgrLogger,
	}, now)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("creating market manager: %v", err)
	}

	positionMgrLogger := logger.With().Str("component", "positionmanager").Logger()
	positionMgr, err = position.NewPositionManager(&position.ManagerConfig{
		Markets: cfg.Markets,
		Notify: func(message string) {
			// todo.
		},
		PersistClosedPosition: func(position *position.Position) error {
			// todo.
			return nil
		},
		JobScheduler: jobScheduler,
		Logger:       &positionMgrLogger,
	})

	levelReactionLeak := make(chan shared.LevelReaction, 10)
	levelReactionFunc := func(signal shared.LevelReaction) {
		if entryEngine != nil {
			levelReactionLeak <- signal
			entryEngine.SignalLevelReaction(signal)
		}
	}

	priceActionMgrLogger := logger.With().Str("component", "priceactionmanager").Logger()
	priceActionMgr, err = priceaction.NewManager(&priceaction.ManagerConfig{
		Markets:             cfg.Markets,
		Subscribe:           fetchMgr.Subscribe,
		RequestPriceData:    marketMgr.SendPriceDataRequest,
		SignalLevelReaction: levelReactionFunc,
		Logger:              &priceActionMgrLogger,
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("creating price action manager: %v", err)
	}

	entrySignalLeak := make(chan shared.EntrySignal, 10)
	entrySignalFunc := func(signal shared.EntrySignal) {
		entrySignalLeak <- signal
		positionMgr.SendEntrySignal(signal)
	}
	engineLogger := logger.With().Str("component", "engine").Logger()
	entryEngine = engine.NewEngine(&engine.EngineConfig{
		RequestCandleMetadata: priceActionMgr.SendCandleMetadataRequest,
		RequestAverageVolume:  marketMgr.SendAverageVolumeRequest,
		SendEntrySignal:       entrySignalFunc,
		SendExitSignal:        positionMgr.SendExitSignal,
		RequestMarketSkew:     positionMgr.SendMarketSkewRequest,
		Logger:                engineLogger,
	})

	service := &Entry{
		cfg:                cfg,
		fetchManager:       fetchMgr,
		marketManager:      marketMgr,
		positionManager:    positionMgr,
		priceActionManager: priceActionMgr,
		historicData:       historicData,
		entryEngine:        entryEngine,
	}

	return service, signalLevelLeak, levelReactionLeak, entrySignalLeak, nil
}

func TestEntryBacktestIntegration(t *testing.T) {
	entry, levelSignals, levelReactions, entrySignals, err := setupEntry()
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		entry.Run(ctx)
		close(done)
	}()

	mkt := "^GSPC"

	// Ensure the backtest creates a level signal.
	levelSig := <-levelSignals
	assert.Equal(t, levelSig.Market, mkt)

	// Ensure the levels created by the backtest have a level reaction.
	reactionSig := <-levelReactions
	assert.Equal(t, reactionSig.Reaction, shared.Reversal)

	// Ensure the level reaction created is high confluence enough to create a long entry from.
	entrySig := <-entrySignals
	assert.Equal(t, entrySig.Direction, shared.Long)

	cancel()
	<-done
}
