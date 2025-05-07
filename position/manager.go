package position

import (
	"context"
	"fmt"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
)

// ManagerConfig represents the position manager configuration.
type ManagerConfig struct {
	// Markets represents the collection of ids of the markets to manage.
	Markets []string
	// Notify sends the provided message.
	Notify func(message string)
	// Backtest is the backtesting flag.
	Backtest bool
	// PersistClosedPosition persists the provided closed position to the database.
	PersistClosedPosition func(position *Position) error
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager manages positions through their lifecycles.
type Manager struct {
	cfg                *ManagerConfig
	markets            map[string]*Market
	entrySignals       chan shared.EntrySignal
	exitSignals        chan shared.ExitSignal
	marketSkewRequests chan shared.MarketSkewRequest
	workers            chan struct{}
}

// NewPositionManager initializes a new position manager.
func NewPositionManager(cfg *ManagerConfig) (*Manager, error) {
	// Create markets for position tracking.
	markets := make(map[string]*Market)
	for idx := range cfg.Markets {
		market := cfg.Markets[idx]
		mCfg := &MarketConfig{
			Market:       market,
			JobScheduler: cfg.JobScheduler,
			Logger:       cfg.Logger,
		}
		mkt, err := NewMarket(mCfg)
		if err != nil {
			return nil, fmt.Errorf("creating new positions market %s: %v", market, err)
		}

		markets[market] = mkt
	}

	return &Manager{
		cfg:                cfg,
		markets:            markets,
		entrySignals:       make(chan shared.EntrySignal, bufferSize),
		exitSignals:        make(chan shared.ExitSignal, bufferSize),
		marketSkewRequests: make(chan shared.MarketSkewRequest, bufferSize),
		workers:            make(chan struct{}, maxWorkers),
	}, nil
}

// SendEntrySignal relays the provided entry signal for processing.
func (m *Manager) SendEntrySignal(signal shared.EntrySignal) {
	select {
	case m.entrySignals <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("entry signal channel at capacity: %d/%d",
			len(m.entrySignals), bufferSize)
	}
}

// SendExitSignal relays the provided exit signal for processing.
func (m *Manager) SendExitSignal(signal shared.ExitSignal) {
	select {
	case m.exitSignals <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("exit signal channel at capacity: %d/%d",
			len(m.exitSignals), bufferSize)
	}
}

// SendMarketSkewRequest relays the provided market skew request for processing.
func (m *Manager) SendMarketSkewRequest(req shared.MarketSkewRequest) {
	select {
	case m.marketSkewRequests <- req:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market skew request channel at capacity: %d/%d",
			len(m.marketSkewRequests), bufferSize)
	}
}

// handleEntrySignal processes the provided entry signal.
func (m *Manager) handleEntrySignal(signal *shared.EntrySignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	position, err := NewPosition(signal)
	if err != nil {
		return fmt.Errorf("creating new position: %v", err)
	}

	mkt, ok := m.markets[position.Market]
	if !ok {
		return fmt.Errorf("no position market found with id %s", position.Market)
	}

	err = mkt.AddPosition(position)
	if err != nil {
		return fmt.Errorf("adding %s position: %v", position.Market, err)
	}

	// Notify of the newly created position.
	msg := fmt.Sprintf("Created new %s position (%s) for %s @ %.2f with stoploss @ %.2f (%.2f points)",
		position.Direction.String(), position.ID, position.Market, position.EntryPrice,
		position.StopLoss, signal.StopLossPointsRange)
	m.cfg.Logger.Info().Msg(msg)
	m.cfg.Notify(msg)

	return nil
}

// handleExitSignal processes the provided exit signal.
func (m *Manager) handleExitSignal(signal *shared.ExitSignal) error {
	defer func() {
		signal.Status <- shared.Processed
	}()

	mkt, ok := m.markets[signal.Market]
	if !ok {
		return fmt.Errorf("no position market found with id %s", signal.Market)
	}

	closedPositions, err := mkt.ClosePositions(signal)
	if err != nil {
		return fmt.Errorf("closing %s position for %s: %v", signal.Direction.String(),
			signal.Market, err)
	}

	for idx := range closedPositions {
		pos := closedPositions[idx]

		m.cfg.PersistClosedPosition(pos)

		// Notify discord session about the closed position.
		msg := fmt.Sprintf("Closed %s position (%s) for %s @ %.2f with stoploss @ %.2f (%.2f points), PNL %.2f",
			pos.Direction.String(), pos.ID, pos.Market, pos.ExitPrice, pos.StopLoss,
			pos.StopLossPointsRange, pos.PNLPercent)
		m.cfg.Logger.Info().Msg(msg)
		m.cfg.Notify(msg)
	}

	return nil
}

// handleMarketSkewRequest processes the provided market skew request.
func (m *Manager) handleMarketSkewRequest(req *shared.MarketSkewRequest) error {
	mkt, ok := m.markets[req.Market]
	if !ok {
		return fmt.Errorf("no position market found with id %s", req.Market)
	}

	req.Response <- shared.MarketSkew(mkt.skew.Load())

	return nil
}

// Run manages the lifecycle processes of the position manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			if !m.cfg.Backtest {
				return
			}

			// Persist tracked positions for markets to csv files when backtesting.
			for k := range m.markets {
				_, err := m.markets[k].PersistPositionsCSV()
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
			}

			return
		case signal := <-m.entrySignals:
			m.workers <- struct{}{}
			go func(signal *shared.EntrySignal) {
				err := m.handleEntrySignal(signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&signal)
		case signal := <-m.exitSignals:
			m.workers <- struct{}{}
			go func(signal *shared.ExitSignal) {
				err := m.handleExitSignal(signal)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&signal)
		case req := <-m.marketSkewRequests:
			m.workers <- struct{}{}
			go func(req *shared.MarketSkewRequest) {
				err := m.handleMarketSkewRequest(req)
				if err != nil {
					m.cfg.Logger.Error().Err(err).Send()
				}
				<-m.workers
			}(&req)
		default:
			// fallthrough
		}
	}
}
