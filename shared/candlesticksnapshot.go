package shared

import (
	"errors"
)

const (
	// SnapshotSize is the maximum number of entries for a candlestick snapshot.
	SnapshotSize = 36
)

// CandlestickSnapshot represents a snapshot of candlestick data.
type CandlestickSnapshot struct {
	data  []*Candlestick
	start int
	count int
	size  int
}

// NewCandlestickSnapshot initializes a new candlestick snapshot.
func NewCandlestickSnapshot(size int) (*CandlestickSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}
	return &CandlestickSnapshot{
		data: make([]*Candlestick, size),
		size: size,
	}, nil
}

// Update adds the provided candlestick to the snapshot.
func (s *CandlestickSnapshot) Update(candle *Candlestick) {
	end := (s.start + s.count) % s.size
	s.data[end] = candle

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start = (s.start + 1) % s.size
	} else {
		s.count++
	}
}

// Last returns the last added entry for the snapshot.
func (s *CandlestickSnapshot) Last() *Candlestick {
	if s.count == 0 {
		return nil
	}

	end := (s.start + s.count - 1) % s.size
	return s.data[end]
}

// LastN fetches the last n number of elements from the snapshot.
func (s *CandlestickSnapshot) LastN(n int) []*Candlestick {
	if n <= 0 {
		return nil
	}

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > s.count {
		n = s.count
	}

	set := make([]*Candlestick, n)
	start := (s.start + s.count - n + s.size) % s.size

	for i := range n {
		idx := (start + i) % s.size
		set[i] = s.data[idx]
	}

	return set
}

// AverageVolumeN returns the average volume for last n candles besides the most recent one.
func (s *CandlestickSnapshot) AverageVolumeN(n int) float64 {
	candles := s.LastN(n + 1)

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > s.count {
		n = s.count
	}

	var volumeSum float64
	for idx := range candles[:n] {
		volumeSum += candles[idx].Volume
	}

	average := volumeSum / float64(n)
	return average
}
