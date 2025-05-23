package shared

import (
	"errors"
	"sync"

	"go.uber.org/atomic"
)

const (
	// ImbalanceSnapshotSize is the maximum number of elements for an imbalance snapshot.
	ImbalanceSnapshotSize = 100
)

// ImbalanceSnapshot represents a snapshot of imbalances for a market.
type ImbalanceSnapshot struct {
	data    []*Imbalance
	dataMtx sync.RWMutex
	start   atomic.Int32
	count   atomic.Int32
	size    atomic.Int32
}

// NewImbalanceSnapshot initializes a new imbalance snapshot.
func NewImbalanceSnapshot(size int32) (*ImbalanceSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &ImbalanceSnapshot{
		data: make([]*Imbalance, size),
	}

	snapshot.size.Store(int32(size))
	return snapshot, nil
}

// Add adds the provided imbalance to the snapshot.
func (s *ImbalanceSnapshot) Add(imb *Imbalance) error {
	s.dataMtx.Lock()
	defer s.dataMtx.Unlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	end := (start + count) % size
	s.data[end] = imb

	if count == size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((start + 1) % size)
	} else {
		s.count.Add(1)
	}

	return nil
}

// Update applies the provided market update to all tracked levels.
func (s *ImbalanceSnapshot) Update(candle *Candlestick) {
	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	for i := range count {
		imbalance := s.data[(start+i)%size]
		imbalance.Update(candle)
	}
}

// Last returns the last added entry for the snapshot.
func (s *ImbalanceSnapshot) Last() *Imbalance {
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
func (s *ImbalanceSnapshot) LastN(n int32) []*Imbalance {
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

	set := make([]*Imbalance, n)
	start = (start + count - n + size) % size

	for i := range n {
		idx := (start + i) % size
		set[i] = s.data[idx]
	}

	return set
}

// Filter applies the provided function to the snapshot and returns the filtered subset.
func (s *ImbalanceSnapshot) Filter(candle *Candlestick, fn func(*Imbalance, *Candlestick) bool) []*Imbalance {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	levels := make([]*Imbalance, 0)
	for i := range count {
		level := s.data[(start+i)%size]
		ok := fn(level, candle)
		if ok {
			levels = append(levels, level)
		}
	}

	return levels
}
