package position

import (
	"context"
	"fmt"
	"slices"
	"sync"

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
	// Notify sends the provided message.
	Notify func(message string)
	// PersistClosedPosition persists the provided closed position to the database.
	PersistClosedPosition func(position *Position) error
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager manages positions through their lifecycles.
type Manager struct {
	cfg          *ManagerConfig
	positions    []*Position
	positionsMtx sync.RWMutex
	entrySignals chan shared.EntrySignal
	exitSignals  chan shared.ExitSignal
	workers      chan struct{}
}

// NewPositionManager initializes a new position manager.
func NewPositionManager(cfg *ManagerConfig) *Manager {
	return &Manager{
		cfg:          cfg,
		positions:    []*Position{},
		entrySignals: make(chan shared.EntrySignal, bufferSize),
		exitSignals:  make(chan shared.ExitSignal, bufferSize),
		workers:      make(chan struct{}, maxWorkers),
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

// handleEntrySignal processes the provided entry signal.
func (m *Manager) handleEntrySignal(signal shared.EntrySignal) {
	defer func() {
		if signal.Done != nil {
			close(signal.Done)
		}
	}()

	position, err := NewPosition(&signal)
	if err != nil {
		m.cfg.Logger.Error().Msgf("creating new position: %v", err)
		return
	}

	m.positionsMtx.Lock()
	m.positions = append(m.positions, position)
	m.positionsMtx.Unlock()

	// Notify of the newly created position.
	msg := fmt.Sprintf("Created new %s position (%s) for %s @ %f with stoploss %f",
		position.Direction.String(), position.ID, position.Market, position.EntryPrice, position.StopLoss)
	m.cfg.Notify(msg)
}

// handleExitSignal processes the provided exit signal.
func (m *Manager) handleExitSignal(signal shared.ExitSignal) {
	for idx, pos := range m.positions {
		switch {
		case pos.Direction == signal.Direction:
			if pos.Market == signal.Market && pos.Timeframe == signal.Timeframe {
				pos.UpdatePNLPercent(signal.Price)
				pos.ClosePosition(&signal)
				m.cfg.PersistClosedPosition(pos)

				// Notify discord session about the closed position.
				msg := fmt.Sprintf("Closed %s position (%s) for %s @ %f with stoploss %f",
					pos.Direction.String(), pos.ID, pos.Market, pos.ExitPrice, pos.StopLoss)
				m.cfg.Notify(msg)

				m.positionsMtx.Lock()
				m.positions = slices.Delete(m.positions, idx, idx+1)
				m.positionsMtx.Unlock()
			}
		default:
			// do nothing.
		}
	}
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
				m.handleEntrySignal(*signal)
				<-m.workers
			}(&signal)
		case signal := <-m.exitSignals:
			m.workers <- struct{}{}
			go func(signal *shared.ExitSignal) {
				m.handleExitSignal(*signal)
				<-m.workers
			}(&signal)
		default:
			// fallthrough
		}
	}
}
