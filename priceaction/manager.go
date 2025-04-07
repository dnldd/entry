package priceaction

import (
	"context"
	"fmt"
	"sync"

	"github.com/dnldd/entry/market"
	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// bufferSize is the default buffer size for channels.
	bufferSize = 64
	// maxWorkers is the maximum number of concurrent workers.
	maxWorkers = 8
)

// ManagerConfig represents the price action manager configuration.
type ManagerConfig struct {
	// Subscribe registers the provided subscriber for market updates.
	Subscribe func(sub *chan shared.Candlestick)
	// Logger represents the application logger.
	Logger zerolog.Logger
}

// Manager represents the price action manager.
type Manager struct {
	cfg               *ManagerConfig
	levelSnapshot     *LevelSnapshot
	levelSignals      chan market.LevelSignal
	updateSignals     chan shared.Candlestick
	currentCandles    map[string]*shared.Candlestick
	currentCandlesMtx sync.RWMutex
	workers           chan struct{}
}

// NewManager initializes a new price action manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	levelSnapshot, err := NewLevelSnapshot()
	if err != nil {
		return nil, fmt.Errorf("creating level snapshot: %v", err)
	}

	return &Manager{
		cfg:            cfg,
		levelSnapshot:  levelSnapshot,
		levelSignals:   make(chan market.LevelSignal, bufferSize),
		updateSignals:  make(chan shared.Candlestick),
		currentCandles: make(map[string]*shared.Candlestick),
		workers:        make(chan struct{}, maxWorkers),
	}, nil
}

// SendLevel relays the provided level signal for processing.
func (m *Manager) SendLevelSignal(level market.LevelSignal) {
	select {
	case m.levelSignals <- level:
		// do nothing.
	default:
		m.cfg.Logger.Error().Msgf("level channel at capacity: %d/%d",
			len(m.levelSignals), bufferSize)
	}
}

// handleUpdateSignal processes the provided update signal.
func (m *Manager) handleUpdateCandle(candle *shared.Candlestick) {
	m.currentCandlesMtx.Lock()
	m.currentCandles[candle.Market] = candle
	m.currentCandlesMtx.Unlock()
}

// handleLevelSignal processes the provided level signal.
func (m *Manager) handleLevelSignal(signal market.LevelSignal) {
	m.currentCandlesMtx.RLock()
	currentCandle := m.currentCandles[signal.Market]
	m.currentCandlesMtx.RUnlock()

	if currentCandle == nil {
		m.cfg.Logger.Error().Msgf("no current candle available, skipping level")
		return
	}

	level := NewLevel(signal.Market, signal.Price, currentCandle)
	m.levelSnapshot.Add(level)
}

// Run manages the lifecycle processes of the price action manager.
func (m *Manager) Run(ctx context.Context) {
	m.cfg.Subscribe(&m.updateSignals)

	for {
		select {
		case signal := <-m.levelSignals:
			m.workers <- struct{}{}
			go func(level *market.LevelSignal) {
				m.handleLevelSignal(signal)
				<-m.workers
			}(&signal)
		case candle := <-m.updateSignals:
			m.workers <- struct{}{}
			go func(candle *shared.Candlestick) {
				m.handleUpdateCandle(candle)
				<-m.workers
			}(&candle)
		default:
			// fallthrough
		}
	}
}
