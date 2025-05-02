package market

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dnldd/entry/indicator"
	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
)

const (
	// updateTimeframe is the expected timeframe for candle updates.
	updateTimeframe = shared.FiveMinute
)

type MarketConfig struct {
	// Market is the name of the tracked market.
	Market string
	// SignalLevel relays the provided level signal for processing.
	SignalLevel func(signal shared.LevelSignal)
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Market tracks the metadata of a market.
type Market struct {
	cfg             *MarketConfig
	candleSnapshot  *shared.CandlestickSnapshot
	sessionSnapshot *SessionSnapshot
	vwap            *indicator.VWAP
	caughtUp        atomic.Bool
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig, now time.Time) (*Market, error) {
	sessionsSnapshot, err := NewSessionSnapshot(shared.SnapshotSize, now)
	if err != nil {
		return nil, err
	}

	candleSnapshot, err := shared.NewCandlestickSnapshot(shared.SnapshotSize)
	if err != nil {
		return nil, err
	}

	mkt := &Market{
		cfg:             cfg,
		candleSnapshot:  candleSnapshot,
		sessionSnapshot: sessionsSnapshot,
		vwap:            indicator.NewVWAP(cfg.Market, shared.FiveMinute),
	}

	// Periodically reset the market vwap when the new york session closes.
	_, err = mkt.cfg.JobScheduler.Every(1).Day().At(indicator.VwapResetTime).WaitForSchedule().
		Do(mkt.vwap.Reset)
	if err != nil {
		return nil, fmt.Errorf("scheduling %s market vwap reset job for %s: %w", mkt.cfg.Market,
			shared.FiveMinute, err)
	}

	// Periodically add sessions covering the day to the snapshot.
	_, err = mkt.cfg.JobScheduler.Every(1).Day().At(SessionGenerationTime).WaitForSchedule().
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
	defer func() {
		candle.Status <- shared.Processed
	}()

	if candle.Timeframe != shared.FiveMinute {
		// do nothing.
		m.cfg.Logger.Info().Msgf("encountered %s candle for updates instead of the expected "+
			"%s timeframe, skipping", candle.Timeframe.String(), updateTimeframe.String())
		return nil
	}

	m.candleSnapshot.Update(candle)
	changed, err := m.sessionSnapshot.SetCurrentSession(candle.Date)
	if err != nil {
		return fmt.Errorf("setting current session: %w", err)
	}

	m.sessionSnapshot.FetchCurrentSession().Update(candle)
	_, err = m.vwap.Update(candle)
	if err != nil {
		return err
	}

	if changed {
		// Fetch and send new high and low from completed sessions.
		high, low, err := m.sessionSnapshot.FetchLastSessionHighLow()
		if err != nil {
			return fmt.Errorf("fetching new levels: %w", err)
		}

		sessionHigh := shared.NewLevelSignal(candle.Market, high)
		m.cfg.SignalLevel(sessionHigh)

		sessionLow := shared.NewLevelSignal(candle.Market, low)
		m.cfg.SignalLevel(sessionLow)
	}

	return nil
}
