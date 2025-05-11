package shared

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/atomic"
)

// VWAPSnapshot represents a snapshot of vwap data.
type VWAPSnapshot struct {
	data    []*VWAP
	dataMtx sync.RWMutex
	start   atomic.Int32
	count   atomic.Int32
	size    atomic.Int32
}

// NewVWAPSnapshot initializes a new vwap snapshot.
func NewVWAPSnapshot(size int32) (*VWAPSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &VWAPSnapshot{
		data: make([]*VWAP, size),
	}
	snapshot.size.Store(int32(size))

	return snapshot, nil
}

// Update adds the provided vwap to the snapshot.
func (s *VWAPSnapshot) Update(vwap *VWAP) {
	s.dataMtx.Lock()
	defer s.dataMtx.Unlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	end := (start + count) % size
	s.data[end] = vwap

	if count == size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((start + 1) % size)
	} else {
		s.count.Add(1)
	}
}

// Last returns the last added entry for the snapshot.
func (s *VWAPSnapshot) Last() *VWAP {
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

// At returns the vwap entry at the provided time for the snapshot.
func (s *VWAPSnapshot) At(t time.Time) *VWAP {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()

	for i := int32(0); i < count; i++ {
		idx := (start + count - 1 - i + size) % size
		if s.data[idx].Date.Equal(t) {
			return s.data[idx]
		}
	}

	return nil
}

// LastN fetches the last n number of elements from the snapshot.
func (s *VWAPSnapshot) LastN(n int32) []*VWAP {
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

	set := make([]*VWAP, n)
	start = (start + count - n + size) % size

	for i := range n {
		idx := (start + i) % size
		set[i] = s.data[idx]
	}

	return set
}
