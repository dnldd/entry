package market

import "github.com/dnldd/entry/shared"

const (
	// snapshotSize is the maximum number of entries for a candlestick snapshot.
	candlestickSnapshotSize = 36
)

// CandlestickSnapshot represents a snapshot of candlestick data.
type CandlestickSnapshot struct {
	data  []*shared.Candlestick
	start int
	count int
	size  int
}

// NewCandlestickSnapshot initializes a new candlestick snapshot.
func NewCandlestickSnapshot() *CandlestickSnapshot {
	return &CandlestickSnapshot{
		data: make([]*shared.Candlestick, candlestickSnapshotSize),
	}
}

// Update adds the provided candlestick to the snapshot.
func (s *CandlestickSnapshot) Update(candle *shared.Candlestick) {
	end := (s.start + s.count) % s.size
	s.data[end] = candle

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start = (s.start + 1) % s.size
	} else {
		s.count++
	}
}

// LastN fetches the last n number of elements from the snapshot.
func (s *CandlestickSnapshot) LastN(n int) []*shared.Candlestick {
	if n <= 0 {
		return nil
	}

	// Clamp the number of elements excpected if it is greater than the snapshot count.
	if n > s.count {
		n = s.count
	}

	set := make([]*shared.Candlestick, n)
	start := (s.start + s.count - n + s.size) % s.size

	for i := range n {
		idx := (start + i) % s.size
		set[i] = s.data[idx]
	}

	return set
}

// AverageVolumeN returns the average volume for all candles besides the most recent one.
func (s *CandlestickSnapshot) AverageVolumeN(n int) float64 {
	candles := s.LastN(n + 1)

	var volumeSum float64
	for idx := range candles[:n] {
		volumeSum += candles[idx].Volume
	}

	average := volumeSum / float64(len(candles))
	return average
}
