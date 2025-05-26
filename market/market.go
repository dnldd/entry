package market

import (
	"errors"
	"fmt"
	"time"

	"github.com/dnldd/entry/indicator"
	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

const (
	// updateTimeframe is the expected timeframe for candle updates.
	updateTimeframe = shared.FiveMinute
)

type MarketConfig struct {
	// Market is the name of the tracked market.
	Market string
	// Timeframes is the timeframes the market is expected to track.
	Timeframes []shared.Timeframe
	// SignalLevel relays the provided level signal for processing.
	SignalLevel func(signal shared.LevelSignal)
	// SignalImbalanace relays the provided imbalance signal for processing.
	SignalImbalance func(signal shared.ImbalanceSignal)
	// RelayMarketUpdate relays the provided market update to the price action
	// manager for processing.
	RelayMarketUpdate func(candle shared.Candlestick)
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Validate asserts the config sane inputs.
func (cfg *MarketConfig) Validate() error {
	var errs error

	if cfg.Market == "" {
		errs = errors.Join(errs, fmt.Errorf("market cannot be an empty string"))
	}
	if len(cfg.Timeframes) == 0 {
		errs = errors.Join(errs, fmt.Errorf("no timeframes provided for market"))
	}
	if cfg.SignalLevel == nil {
		errs = errors.Join(errs, fmt.Errorf("signal level function cannot be nil"))
	}
	if cfg.SignalImbalance == nil {
		errs = errors.Join(errs, fmt.Errorf("signal imbalance function cannot be nil"))
	}
	if cfg.RelayMarketUpdate == nil {
		errs = errors.Join(errs, fmt.Errorf("relay market update function cannot be nil"))
	}
	if cfg.JobScheduler == nil {
		errs = errors.Join(errs, fmt.Errorf("job scheduler cannot be nil"))
	}
	if cfg.Logger == nil {
		errs = errors.Join(errs, fmt.Errorf("logger cannot be nil"))
	}

	return errs
}

// Market tracks the metadata of a market.
//
// The market tracks candlestick data spanning multiple timeframes – 1m, 5m & 1H,
// as well as their corresponding vwap indicators and vwap snapshots.
type Market struct {
	cfg             *MarketConfig
	sessionSnapshot *shared.SessionSnapshot
	candleSnapshots map[shared.Timeframe]*shared.CandlestickSnapshot
	vwapSnapshots   map[shared.Timeframe]*shared.VWAPSnapshot
	vwapIndicators  map[shared.Timeframe]*indicator.VWAP
	caughtUp        atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig, now time.Time) (*Market, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("validating market config: %v", err)
	}

	sessionsSnapshot, err := shared.NewSessionSnapshot(shared.SessionSnapshotSize, now)
	if err != nil {
		return nil, err
	}

	// Create candlestick snapshots for all tracked timeframes.
	candleSnapshots := make(map[shared.Timeframe]*shared.CandlestickSnapshot)
	for idx := range cfg.Timeframes {
		timeframe := cfg.Timeframes[idx]

		switch timeframe {
		case shared.OneMinute:
			snapshot, err := shared.NewCandlestickSnapshot(shared.OneMinuteSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			candleSnapshots[timeframe] = snapshot
		case shared.FiveMinute:
			snapshot, err := shared.NewCandlestickSnapshot(shared.FiveMinuteSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			candleSnapshots[timeframe] = snapshot
		case shared.OneHour:
			snapshot, err := shared.NewCandlestickSnapshot(shared.OneHourSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			candleSnapshots[timeframe] = snapshot
		}
	}

	// Create vwap snapshots for all tracked timeframes.
	vwapSnapshots := make(map[shared.Timeframe]*shared.VWAPSnapshot)
	for idx := range cfg.Timeframes {
		timeframe := cfg.Timeframes[idx]

		switch timeframe {
		case shared.OneMinute:
			snapshot, err := shared.NewVWAPSnapshot(shared.OneMinuteSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			vwapSnapshots[timeframe] = snapshot
		case shared.FiveMinute:
			snapshot, err := shared.NewVWAPSnapshot(shared.FiveMinuteSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			vwapSnapshots[timeframe] = snapshot
		case shared.OneHour:
			snapshot, err := shared.NewVWAPSnapshot(shared.OneHourSnapshotSize, timeframe)
			if err != nil {
				return nil, err
			}

			vwapSnapshots[timeframe] = snapshot
		}
	}

	vwapIndicators := make(map[shared.Timeframe]*indicator.VWAP)
	for idx := range cfg.Timeframes {
		timeframe := cfg.Timeframes[idx]

		switch timeframe {
		case shared.OneMinute:
			indicator := indicator.NewVWAP(cfg.Market, timeframe)
			vwapIndicators[timeframe] = indicator
		case shared.FiveMinute:
			indicator := indicator.NewVWAP(cfg.Market, timeframe)
			vwapIndicators[timeframe] = indicator
		case shared.OneHour:
			indicator := indicator.NewVWAP(cfg.Market, timeframe)
			vwapIndicators[timeframe] = indicator
		}
	}

	mkt := &Market{
		cfg:             cfg,
		sessionSnapshot: sessionsSnapshot,
		candleSnapshots: candleSnapshots,
		vwapSnapshots:   vwapSnapshots,
		vwapIndicators:  vwapIndicators,
	}

	// Periodically reset the market vwaps on all timeframes when the new york session closes.
	for idx := range cfg.Timeframes {
		timeframe := cfg.Timeframes[idx]

		vwap := mkt.vwapIndicators[timeframe]
		_, err = mkt.cfg.JobScheduler.Every(1).Day().At(indicator.VwapResetTime).WaitForSchedule().
			Do(vwap.Reset)
		if err != nil {
			return nil, fmt.Errorf("scheduling %s market vwap reset job for timefram %s: %w",
				vwap.Market, vwap.Timeframe, err)
		}
	}

	// Periodically add sessions covering the day to the snapshot.
	_, err = mkt.cfg.JobScheduler.Every(1).Day().At(shared.SessionGenerationTime).WaitForSchedule().
		Do(mkt.sessionSnapshot.GenerateNewSessionsJob, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("scheduling %s market vwap reset job for %s: %w", mkt.cfg.Market,
			shared.FiveMinute, err)
	}

	return mkt, nil
}

// SetCaughtUpStatus updates the caught up status of the provided market.
func (m *Market) SetCaughtUpStatus(status bool) {
	m.caughtUp.Store(status)
}

// CaughtUp returns whether the provided market has caught up on market data.
func (m *Market) CaughtUp() bool {
	return m.caughtUp.Load()
}

// Update processes incoming market data for the provided market.
func (m *Market) Update(candle *shared.Candlestick) error {
	// Update the candle snapshot for the provided timeframe.
	candleSnapshot, ok := m.candleSnapshots[candle.Timeframe]
	if !ok {
		return fmt.Errorf("no candle snapshot found for timeframe %s", candle.Timeframe.String())
	}

	candleSnapshot.Update(candle)

	// Generate the vwap for the provided timeframe.
	indicator, ok := m.vwapIndicators[candle.Timeframe]
	if !ok {
		return fmt.Errorf("no vwap indicator found for timeframe %s", candle.Timeframe.String())
	}

	vwap, err := indicator.Update(candle)
	if err != nil {
		return fmt.Errorf("updating vwap indicator for market %s at timeframe %s",
			indicator.Market, indicator.Timeframe)
	}

	// Update the vwap snapshot for the provided timeframe.
	vwapSnapshot, ok := m.vwapSnapshots[candle.Timeframe]
	if !ok {
		return fmt.Errorf("no vwap snapshot found for timeframe %s", candle.Timeframe.String())
	}

	vwapSnapshot.Update(vwap)

	// Notify the price action manager of the received market update.
	updateCandle := *candle
	updateCandle.Status = make(chan shared.StatusCode, 1)

	m.cfg.RelayMarketUpdate(updateCandle)
	select {
	case <-updateCandle.Status:
	case <-time.After(shared.TimeoutDuration):
		return fmt.Errorf("timed out while waiting for market update status")
	}

	// Only generate level and imbalance signals on the 5m timeframe.
	if candle.Timeframe == shared.FiveMinute {
		// Detect and send imbalances.
		imbalance, ok := candleSnapshot.DetectImbalance()
		if ok {
			imbalanaceSignal := shared.NewImbalanceSignal(candle.Market, *imbalance)
			m.cfg.SignalImbalance(imbalanaceSignal)
			select {
			case <-imbalanaceSignal.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out while waiting for imbalance signal status")
			}
		}

		changed, err := m.sessionSnapshot.SetCurrentSession(candle.Date)
		if err != nil {
			return fmt.Errorf("setting current session: %w", err)
		}

		m.sessionSnapshot.FetchCurrentSession().Update(candle)

		if changed {
			// Fetch and send new high and low from completed sessions.
			high, low, err := m.sessionSnapshot.FetchLastSessionHighLow()
			if err != nil {
				return fmt.Errorf("fetching new levels: %w", err)
			}

			// Skip new level signals if either high or low from the last session is zero.
			if high == 0 || low == 0 {
				m.cfg.Logger.Info().Msgf("high and low cannot be zero, skipping new level signal")
				return nil
			}

			sessionHigh := shared.NewLevelSignal(candle.Market, high, candle.Close)
			m.cfg.SignalLevel(sessionHigh)
			select {
			case <-sessionHigh.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out while waiting for level signal status")
			}

			sessionLow := shared.NewLevelSignal(candle.Market, low, candle.Close)
			m.cfg.SignalLevel(sessionLow)
			select {
			case <-sessionLow.Status:
			case <-time.After(shared.TimeoutDuration):
				return fmt.Errorf("timed out while waiting for level signal status")
			}
		}
	}

	return nil
}
