package main

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/rs/zerolog"
)

// bufferSize is the default buffer size for channels.
const bufferSize = 64

// PositionManagerConfig represents the position manager configuration.
type PositionManagerConfig struct {
	// Notify sends the provided message.
	Notify func(message string)
	// PersistClosedPosition persists the provided closed position to the database.
	PersistClosedPosition func(position *Position) error
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// PositionManager manages positions through their lifecycles.
type PositionManager struct {
	cfg          *PositionManagerConfig
	positions    []*Position
	positionsMtx sync.RWMutex
	entrySignals chan EntrySignal
	exitSignals  chan ExitSignal
}

// NewPositionManager initializes a new position manager.
func NewPositionManager(cfg *PositionManagerConfig) *PositionManager {
	return &PositionManager{
		cfg:          cfg,
		positions:    []*Position{},
		entrySignals: make(chan EntrySignal, bufferSize),
		exitSignals:  make(chan ExitSignal, bufferSize),
	}
}

// SendEntrySignal relays the provided entry signal for processing.
func (m *PositionManager) SendEntrySignal(signal EntrySignal) {
	select {
	case m.entrySignals <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("entry signal channel at capacity: %d/%d",
			len(m.entrySignals), bufferSize)
	}
}

// SendExitSignal relays the provided exit signal for processing.
func (m *PositionManager) SendExitSignal(signal ExitSignal) {
	select {
	case m.exitSignals <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("exit signal channel at capacity: %d/%d",
			len(m.exitSignals), bufferSize)
	}
}

// handleEntrySignal processes the provided entry signal.
func (m *PositionManager) handleEntrySignal(signal EntrySignal) {
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
func (m *PositionManager) handleExitSignal(signal ExitSignal) {
	for idx, pos := range m.positions {
		switch {
		case pos.Direction == signal.Direction:
			if pos.Market == signal.Market && pos.Timeframe == signal.Timeframe {
				pos.UpdatePNLPercent(signal.Price)
				pos.ClosePosition(&signal)

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
func (m *PositionManager) Run(ctx context.Context) {
	for {
		select {
		// todo: add entry and exit signal workers.
		case signal := <-m.entrySignals:
			m.handleEntrySignal(signal)
		case signal := <-m.exitSignals:
			m.handleExitSignal(signal)
		default:
			// fallthrough
		}
	}
}
