package priceaction

import "github.com/dnldd/entry/shared"

const (
	// snapshotSize is the maximum number of elements for a level snapshot.
	levelSnapshotSize = 80
)

// LevelSnapshot represents a snapshot of level data.
type LevelSnapshot struct {
	data  []*shared.Level
	start int
	count int
	size  int
}

// NewLevelSnapshot initializes a new level snapshot.
func NewLevelSnapshot() (*LevelSnapshot, error) {
	snapshot := &LevelSnapshot{
		data: make([]*shared.Level, levelSnapshotSize),
	}
	return snapshot, nil
}

// Adds adds the provided session to the snapshot.
func (s *LevelSnapshot) Add(level *shared.Level) {
	end := (s.start + s.count) % s.size
	s.data[end] = level

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start = (s.start + 1) % s.size
	} else {
		s.count++
	}
}

// Filter applies the provided function to the snapshot and returns the filtered subset.
func (s *LevelSnapshot) Filter(candle *shared.Candlestick, fn func(*shared.Level, *shared.Candlestick) bool) []*shared.Level {
	levels := make([]*shared.Level, 0)
	for i := range s.count {
		level := s.data[(s.start+i)%s.size]
		ok := fn(level, candle)
		if ok {
			levels = append(levels, level)
		}
	}

	return levels
}
