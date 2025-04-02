package main

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/rs/zerolog"
)

const (
	// bufferSize is the buffer size for channels used by the manager.
	bufferSize = 64
)

// ManagerConfig represents the manager configuration.
type ManagerConfig struct {
	// Notify sends the provided message.
	Notify func(message string) error
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// EntrySignal represents an entry signal for a market position.
type EntrySignal struct {
	Market        string
	Timeframe     string
	Direction     Direction
	EntryPrice    float64
	EntryCriteria string
	StopLoss      float64
}

// ExitSignal represents an exit signal for a market position.
type ExitSignal struct {
	Market       string
	Timeframe    string
	Direction    Direction
	ExitPrice    float64
	ExitCriteria string
}

// Manager is the hub of the system, it recieves entry and exit signals and executes them.
type Manager struct {
	cfg          *ManagerConfig
	entryCh      chan EntrySignal
	exitCh       chan ExitSignal
	positions    []*Position
	positionsMtx sync.RWMutex
}

// NewManager initializes a new manager.
func NewManager(cfg *ManagerConfig) *Manager {
	return &Manager{
		cfg:       cfg,
		positions: make([]*Position, 0, bufferSize),
		entryCh:   make(chan EntrySignal, bufferSize),
		exitCh:    make(chan ExitSignal, bufferSize),
	}
}

// SendEntrySignal relays the provided entry signal for processing.
func (m *Manager) SendEntrySignal(signal EntrySignal) {
	select {
	case m.entryCh <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("entry signal channel at capacity: %d/%d",
			len(m.entryCh), bufferSize)
	}
}

// SendExitSignal relays the provided exit signal for processing.
func (m *Manager) SendExitSignal(signal EntrySignal) {
	select {
	case m.entryCh <- signal:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("exit signal channel at capacity: %d/%d",
			len(m.exitCh), bufferSize)
	}
}

// handleEntrySignal processes the provided entry signal.
func (m *Manager) handleEntrySignal(signal EntrySignal) {
	position := NewPosition(signal.Market, signal.Timeframe, signal.Direction,
		signal.EntryPrice, signal.EntryCriteria, signal.StopLoss)

	m.positionsMtx.Lock()
	m.positions = append(m.positions, position)
	m.positionsMtx.Unlock()

	// Notify discord of the newly created position.
	msg := fmt.Sprintf("Created new %s position (%s) for %s @ %f with stoploss %f",
		position.Direction.String(), position.ID, position.Market, position.EntryPrice, position.StopLoss)
	m.cfg.Notify(msg)
}

// handleExitSignal processes the provided exit signal.
func (m *Manager) handleExitSignal(signal ExitSignal) {
	for idx, pos := range m.positions {
		switch {
		case pos.Direction == signal.Direction:
			if pos.Market == signal.Market && pos.Timeframe == signal.Timeframe {
				pos.UpdatePNLPercent(signal.ExitPrice)
				pos.ClosePosition(signal.ExitPrice, signal.ExitCriteria)

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

// Run manages the lifecycle processes of the manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case signal := <-m.entryCh:
			go m.handleEntrySignal(signal)
		case signal := <-m.exitCh:
			go m.handleExitSignal(signal)
		default:
			// fallthrough.
		}
	}
}
