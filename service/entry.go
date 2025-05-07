package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnldd/entry/engine"
	"github.com/dnldd/entry/fetch"
	"github.com/dnldd/entry/market"
	"github.com/dnldd/entry/position"
	"github.com/dnldd/entry/priceaction"
	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

// EntryConfig represents the configuration struct for the entry service.
type EntryConfig struct {
	// Markets represents the tracked markets.
	Markets []string
	// FMPAPIkey is the FMP service API Key.
	FMPAPIKey string
	// Backtest is the backtesting flag.
	Backtest bool
	// BacktestMaret is the market being backtested.
	BacktestMarket string
	// BacktestDataFilepath is the filepath to the backtest data.
	BacktestDataFilepath string
}

// Entry represents a market entry finding service.
type Entry struct {
	cfg                *EntryConfig
	fetchManager       *fetch.Manager
	marketManager      *market.Manager
	positionManager    *position.Manager
	priceActionManager *priceaction.Manager
	historicData       *shared.HistoricData
	entryEngine        *engine.Engine
	wg                 sync.WaitGroup
}

// NewEntry initializes a new entry service.
func NewEntry(cfg *EntryConfig) (*Entry, error) {
	var err error
	var marketMgr *market.Manager
	var fetchMgr *fetch.Manager
	var positionMgr *position.Manager
	var priceActionMgr *priceaction.Manager
	var historicData *shared.HistoricData
	var entryEngine *engine.Engine

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
		return nil, fmt.Errorf("fetching new york time: %v", err)
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
			return nil, fmt.Errorf("creating historic data: %v", err)
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
		return nil, fmt.Errorf("creating fetch manager: %v", err)
	}

	signalLevelFunc := func(signal shared.LevelSignal) {
		if priceActionMgr != nil {
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
		return nil, fmt.Errorf("creating market manager: %v", err)
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

	levelReactionFunc := func(signal shared.LevelReaction) {
		if entryEngine != nil {
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
		return nil, fmt.Errorf("creating price action manager: %v", err)
	}

	engineLogger := logger.With().Str("component", "engine").Logger()
	entryEngine = engine.NewEngine(&engine.EngineConfig{
		RequestCandleMetadata: priceActionMgr.SendCandleMetadataRequest,
		RequestAverageVolume:  marketMgr.SendAverageVolumeRequest,
		SendEntrySignal:       positionMgr.SendEntrySignal,
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

	return service, nil
}

// Run handles the lifecycle processes of the entry service.
func (e *Entry) Run(ctx context.Context) {
	e.wg.Add(5)

	go func() {
		e.positionManager.Run(ctx)
		e.wg.Done()
	}()

	go func() {
		e.priceActionManager.Run(ctx)
		e.wg.Done()
	}()

	go func() {
		e.marketManager.Run(ctx)
		e.wg.Done()
	}()

	go func() {
		e.entryEngine.Run(ctx)
		e.wg.Done()
	}()

	go func() {
		e.fetchManager.Run(ctx)
		e.wg.Done()
	}()

	if e.cfg.Backtest {
		go func() {
			// wait briefly.
			time.Sleep(time.Second * 1)
			e.historicData.ProcessHistoricalData()
		}()
	}

	e.wg.Wait()
}
