package shared

import (
	"errors"
	"sync"

	"go.uber.org/atomic"
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

// Update adds the provided imbalance to the snapshot.
func (s *ImbalanceSnapshot) Update(imb *Imbalance) {
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

// Filter returns the entries from the provided snapshot that passes the provided filter function.
func (s *ImbalanceSnapshot) Filter(fn func(*Imbalance) bool) []*Imbalance {
	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

	start := int(s.start.Load())
	count := int(s.count.Load())
	size := int(s.size.Load())

	if count == 0 || size == 0 {
		return nil
	}

	set := make([]*Imbalance, 0, count)
	for idx := range count {
		index := (start + count - 1 - idx + size) % size
		imb := s.data[index]
		if imb != nil {
			if fn(imb) {
				set = append(set, imb)
			}
		}
	}

	return set
}
