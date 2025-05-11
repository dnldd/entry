package shared

import (
	"errors"
	"sync"

	"go.uber.org/atomic"
)

const (
	// SnapshotSize is the maximum number of entries for a candlestick snapshot.
	SnapshotSize = 36
)

// CandlestickSnapshot represents a snapshot of candlestick data.
type CandlestickSnapshot struct {
	data    []*Candlestick
	dataMtx sync.RWMutex
	start   atomic.Int32
	count   atomic.Int32
	size    atomic.Int32
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
	s.dataMtx.Lock()
	defer s.dataMtx.Unlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	end := (start + count) % size
	s.data[end] = candle

	if count == size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((start + 1) % size)
	} else {
		s.count.Add(1)
	}
}

// Last returns the last added entry for the snapshot.
func (s *CandlestickSnapshot) Last() *Candlestick {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	if count == 0 {
		return nil
	}

	end := (start + count - 1) % size
	return s.data[end]
}

// LastN fetches the last n number of elements from the snapshot.
func (s *CandlestickSnapshot) LastN(n int32) []*Candlestick {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	if n <= 0 {
		return nil
	}

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > count {
		n = count
	}

	set := make([]*Candlestick, n)
	start = (start + count - n + size) % size

	for i := range n {
		idx := (start + i) % size
		set[i] = s.data[idx]
	}

	return set
}

// AverageVolumeN returns the average volume for last n candles besides the most recent one.
func (s *CandlestickSnapshot) AverageVolumeN(n int32) float64 {
	candles := s.LastN(n + 1)

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	count := s.count.Load()
	if n > count {
		n = count
	}

	var volumeSum float64
	for idx := range candles[:n] {
		volumeSum += candles[idx].Volume
	}

	average := volumeSum / float64(n)
	return average
}
