package shared

import (
	"errors"
	"sync"

	"go.uber.org/atomic"
)

const (
	// LevelSnapshotSize is the maximum number of elements for a level snapshot.
	LevelSnapshotSize = 80
)

// LevelSnapshot represents a snapshot of level data.
type LevelSnapshot struct {
	data    []*Level
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
		data: make([]*Level, size),
	}

	snapshot.size.Store(size)

	return snapshot, nil
}

// Adds adds the provided session to the snapshot.
func (s *LevelSnapshot) Add(level *Level) {
	s.dataMtx.Lock()
	defer s.dataMtx.Unlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	end := (start + count) % size
	s.data[end] = level

	if count == size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((start + 1) % size)
	} else {
		s.count.Add(1)
	}
}

// Update applies the provided market update to all tracked levels.
func (s *LevelSnapshot) Update(candle *Candlestick) {
	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	for i := range count {
		level := s.data[(start+i)%size]
		level.Update(candle)
	}
}

// Filter applies the provided function to the snapshot and returns the filtered subset.
func (s *LevelSnapshot) Filter(candle *Candlestick, fn func(*Level, *Candlestick) bool) []*Level {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	levels := make([]*Level, 0)
	for i := range count {
		level := s.data[(start+i)%size]
		ok := fn(level, candle)
		if ok {
			levels = append(levels, level)
		}
	}

	return levels
}
