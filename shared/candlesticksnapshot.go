package shared

import (
	"errors"
	"sync/atomic"
)

const (
	// SnapshotSize is the maximum number of entries for a candlestick snapshot.
	SnapshotSize = 36
)

// CandlestickSnapshot represents a snapshot of candlestick data.
type CandlestickSnapshot struct {
	data  []*Candlestick
	start atomic.Int32
	count atomic.Int32
	size  atomic.Int32
}

// NewCandlestickSnapshot initializes a new candlestick snapshot.
func NewCandlestickSnapshot(size int32) (*CandlestickSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &CandlestickSnapshot{
		data: make([]*Candlestick, size),
	}

	snapshot.size.Store(int32(size))
	return snapshot, nil
}

// Update adds the provided candlestick to the snapshot.
func (s *CandlestickSnapshot) Update(candle *Candlestick) {
	end := (s.start.Load() + s.count.Load()) % s.size.Load()
	s.data[end] = candle

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((s.start.Load() + 1) % s.size.Load())
	} else {
		s.count.Add(1)
	}
}

// Last returns the last added entry for the snapshot.
func (s *CandlestickSnapshot) Last() *Candlestick {
	if s.count.Load() == 0 {
		return nil
	}

	end := (s.start.Load() + s.count.Load() - 1) % s.size.Load()
	return s.data[end]
}

// LastN fetches the last n number of elements from the snapshot.
func (s *CandlestickSnapshot) LastN(n int32) []*Candlestick {
	if n <= 0 {
		return nil
	}

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > s.count.Load() {
		n = s.count.Load()
	}

	set := make([]*Candlestick, n)
	start := (s.start.Load() + s.count.Load() - n + s.size.Load()) % s.size.Load()

	for i := range n {
		idx := (start + i) % s.size.Load()
		set[i] = s.data[idx]
	}

	return set
}

// AverageVolumeN returns the average volume for last n candles besides the most recent one.
func (s *CandlestickSnapshot) AverageVolumeN(n int32) float64 {
	candles := s.LastN(n + 1)

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > s.count.Load() {
		n = s.count.Load()
	}

	var volumeSum float64
	for idx := range candles[:n] {
		volumeSum += candles[idx].Volume
	}

	average := volumeSum / float64(n)
	return average
}
