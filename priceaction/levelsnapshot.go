package priceaction

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/dnldd/entry/shared"
)

const (
	// snapshotSize is the maximum number of elements for a level snapshot.
	levelSnapshotSize = 80
)

// LevelSnapshot represents a snapshot of level data.
type LevelSnapshot struct {
	data    []*shared.Level
	dataMtx sync.RWMutex
	start   atomic.Int32
	count   atomic.Int32
	size    atomic.Int32
}

// NewLevelSnapshot initializes a new level snapshot.
func NewLevelSnapshot(size int32) (*LevelSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &LevelSnapshot{
		data: make([]*shared.Level, size),
	}

	snapshot.size.Store(size)

	return snapshot, nil
}

// Adds adds the provided session to the snapshot.
func (s *LevelSnapshot) Add(level *shared.Level) {
	s.dataMtx.Lock()
	defer s.dataMtx.Unlock()

	end := (s.start.Load() + s.count.Load()) % s.size.Load()
	s.data[end] = level

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((s.start.Load() + 1) % s.size.Load())
	} else {
		s.count.Add(1)
	}
}

// Update applies the provided market update to all tracked levels.
func (s *LevelSnapshot) Update(candle *shared.Candlestick) {
	for i := range s.count.Load() {
		level := s.data[(s.start.Load()+i)%s.size.Load()]
		level.Update(candle)
	}
}

// Filter applies the provided function to the snapshot and returns the filtered subset.
func (s *LevelSnapshot) Filter(candle *shared.Candlestick, fn func(*shared.Level, *shared.Candlestick) bool) []*shared.Level {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	levels := make([]*shared.Level, 0)
	for i := range s.count.Load() {
		level := s.data[(s.start.Load()+i)%s.size.Load()]
		ok := fn(level, candle)
		if ok {
			levels = append(levels, level)
		}
	}

	return levels
}
