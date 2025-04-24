package position

import (
	"context"
	"fmt"

	"github.com/dnldd/entry/shared"
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
	// MarketIDs represents the collection of ids of the markets to manage.
	MarketIDs []string
	// Notify sends the provided message.
	Notify func(message string)
	// PersistClosedPosition persists the provided closed position to the database.
	PersistClosedPosition func(position *Position) error
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Manager manages positions through their lifecycles.
type Manager struct {
	cfg                  *ManagerConfig
	markets              map[string]*Market
	entrySignals         chan shared.EntrySignal
	exitSignals          chan shared.ExitSignal
	marketStatusRequests chan shared.MarketStatusRequest
	workers              chan struct{}
}

// NewPositionManager initializes a new position manager.
func NewPositionManager(cfg *ManagerConfig) *Manager {
	// Create markets for position tracking.
	markets := make(map[string]*Market)
	for idx := range cfg.MarketIDs {
		mkt := NewMarket(cfg.MarketIDs[idx])
		markets[mkt.market] = mkt
	}

	return &Manager{
		cfg:                  cfg,
		markets:              markets,
		entrySignals:         make(chan shared.EntrySignal, bufferSize),
		exitSignals:          make(chan shared.ExitSignal, bufferSize),
		marketStatusRequests: make(chan shared.MarketStatusRequest, bufferSize),
		workers:              make(chan struct{}, maxWorkers),
	}
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

// SendMarketStatusRequest relays the provided market status request for processing.
func (m *Manager) SendMarketStatusRequest(req shared.MarketStatusRequest) {
	select {
	case m.marketStatusRequests <- req:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("market status request channel at capacity: %d/%d",
			len(m.marketStatusRequests), bufferSize)
	}
}

// handleEntrySignal processes the provided entry signal.
func (m *Manager) handleEntrySignal(signal *shared.EntrySignal) {
	defer func() {
		if signal.Done != nil {
			close(signal.Done)
		}
	}()

	position, err := NewPosition(signal)
	if err != nil {
		m.cfg.Logger.Error().Msgf("creating new position: %v", err)
		return
	}

	mkt, ok := m.markets[position.Market]
	if !ok {
		m.cfg.Logger.Error().Msgf("no position market found with id %s", position.Market)
		return
	}

	err = mkt.AddPosition(position)
	if err != nil {
		m.cfg.Logger.Error().Msgf("adding %s position: %v", position.Market, err)
		return
	}

	// Notify of the newly created position.
	msg := fmt.Sprintf("Created new %s position (%s) for %s @ %f with stoploss %f",
		position.Direction.String(), position.ID, position.Market, position.EntryPrice, position.StopLoss)
	m.cfg.Notify(msg)
}

// handleExitSignal processes the provided exit signal.
func (m *Manager) handleExitSignal(signal *shared.ExitSignal) {
	mkt, ok := m.markets[signal.Market]
	if !ok {
		m.cfg.Logger.Error().Msgf("no position market found with id %s", signal.Market)
		return
	}

	closedPositions, err := mkt.ClosePositions(signal)
	if err != nil {
		m.cfg.Logger.Error().Msgf("closing %s position for %s: %v", signal.Direction.String(),
			signal.Market, err)
		return
	}

	for idx := range closedPositions {
		pos := closedPositions[idx]

		m.cfg.PersistClosedPosition(pos)

		// Notify discord session about the closed position.
		msg := fmt.Sprintf("Closed %s position (%s) for %s @ %f with stoploss %f",
			pos.Direction.String(), pos.ID, pos.Market, pos.ExitPrice, pos.StopLoss)
		m.cfg.Notify(msg)
	}
}

// handleMarketStatusRequest processes the provided market status request.
func (m *Manager) handleMarketStatusRequest(req *shared.MarketStatusRequest) {
	mkt, ok := m.markets[req.Market]
	if !ok {
		m.cfg.Logger.Error().Msgf("no position market found with id %s", req.Market)
		return
	}

	req.Response <- shared.MarketStatus(mkt.status.Load())
}

// Run manages the lifecycle processes of the position manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-m.entrySignals:
			m.workers <- struct{}{}
			go func(signal *shared.EntrySignal) {
				m.handleEntrySignal(signal)
				<-m.workers
			}(&signal)
		case signal := <-m.exitSignals:
			m.workers <- struct{}{}
			go func(signal *shared.ExitSignal) {
				m.handleExitSignal(signal)
				<-m.workers
			}(&signal)
		case req := <-m.marketStatusRequests:
			m.workers <- struct{}{}
			go func(req *shared.MarketStatusRequest) {
				m.handleMarketStatusRequest(req)
				<-m.workers
			}(req)
		default:
			// fallthrough
		}
	}
}
