package main

import (
	"context"

	"github.com/rs/zerolog"
)

const (
	// channelBufferSize is the buffer size for channels used by the manager.
	channelBufferSize = 64
)

// ManagerConfig represents the manager configuration.
type ManagerConfig struct {
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
	cfg       *ManagerConfig
	entryCh   chan EntrySignal
	exitCh    chan ExitSignal
	positions []*Position
}

// NewManager initializes a new manager.
func NewManager(cfg *ManagerConfig) *Manager {
	return &Manager{
		cfg:       cfg,
		positions: make([]*Position, 0),
		entryCh:   make(chan EntrySignal, channelBufferSize),
		exitCh:    make(chan ExitSignal, channelBufferSize),
	}
}

// SendEntrySignal relays the provided entry signal for processing.
func (m *Manager) SendEntrySignal(signal EntrySignal) {
	select {
	case m.entryCh <- signal:
		// do nothing.
	default:
		// fallthrough.
	}
}

// SendExitSignal relays the provided exit signal for processing.
func (m *Manager) SendExitSignal(signal EntrySignal) {
	select {
	case m.entryCh <- signal:
		// do nothing.
	default:
		// fallthrough.
	}
}

func (m *Manager) handleEntrySignal(ctx context.Context, signal EntrySignal) {}

func (m *Manager) handleExitSignal(ctx context.Context, signal ExitSignal) {}

// Run manages the lifecycle processes of the manager.
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case signal := <-m.entryCh:
			m.handleEntrySignal(ctx, signal)
			// Process entry signal.
		case signal := <-m.exitCh:
			m.handleExitSignal(ctx, signal)
			// Process exit signal.
		default:
			// fallthrough.
		}
	}
}
