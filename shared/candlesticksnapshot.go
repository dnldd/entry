package shared

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"
)

const (
	// SnapshotSize is the maximum number of entries for a candlestick snapshot.
	SnapshotSize = 36
	// minImbalanceRatioThreshold is the minimum imbalance ratio to be considered substantive
	minImbalanceRatioThreshold = 0.24
)

// CandlestickSnapshot represents a snapshot of candlestick data.
type CandlestickSnapshot struct {
	data      []*Candlestick
	dataMtx   sync.RWMutex
	timeframe Timeframe
	start     atomic.Int32
	count     atomic.Int32
	size      atomic.Int32
}

// NewCandlestickSnapshot initializes a new candlestick snapshot.
func NewCandlestickSnapshot(size int32, timeframe Timeframe) (*CandlestickSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &CandlestickSnapshot{
		timeframe: timeframe,
		data:      make([]*Candlestick, size),
	}

	snapshot.size.Store(int32(size))
	return snapshot, nil
}

// Update adds the provided candlestick to the snapshot.
func (s *CandlestickSnapshot) Update(candle *Candlestick) error {
	if candle.Timeframe != s.timeframe {
		return fmt.Errorf("cannot update candlestick snapshot of timeframe %s "+
			"with candle of timeframe %s", s.timeframe, candle.Timeframe)
	}

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

	return nil
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

// DetectImbalance detects an imbalance through from the provided snapshot.
func (s *CandlestickSnapshot) DetectImbalance() (*Imbalance, bool) {
	// Three candles are needed to detect an imbalance.
	candles := s.LastN(3)
	if len(candles) < 3 {
		return nil, false
	}

	avgVolume := s.AverageVolumeN(10)

	firstCandle := candles[0]
	secondCandle := candles[1]
	thirdCandle := candles[2]

	// An imbalance requires a displacement candle with above-average volume,
	// and the candle must be either a marubozu or a pinbar.
	if (secondCandle.FetchKind() != Marubozu && secondCandle.FetchKind() != Pinbar) || secondCandle.Volume < avgVolume {
		return nil, false
	}

	// An imbalance must have an inefficiency - a gap.
	sentiment := secondCandle.FetchSentiment()
	switch sentiment {
	case Bullish:
		displacementSize := secondCandle.Close - secondCandle.Open
		if displacementSize <= 0 {
			return nil, false
		}

		gap := thirdCandle.Low - firstCandle.High

		// A prominent imbalance should be at least 24% of the displacement candle.
		gapRatio := gap / displacementSize
		if gapRatio < minImbalanceRatioThreshold {
			return nil, false
		}

		high := firstCandle.High
		low := thirdCandle.Low
		midpoint := (high + low) / 2

		imbalance := NewImbalance(firstCandle.Market, high, midpoint, low, sentiment, gapRatio, thirdCandle.Date)

		return imbalance, true

	case Bearish:
		displacementSize := secondCandle.Open - secondCandle.Close
		if displacementSize <= 0 {
			return nil, false
		}

		gap := firstCandle.Low - thirdCandle.High

		// A prominent imbalance should be at least 24% of the displacement candle.
		gapRatio := gap / displacementSize
		if gapRatio < minImbalanceRatioThreshold {
			return nil, false
		}

		high := firstCandle.Low
		low := thirdCandle.High
		midpoint := (high + low) / 2

		imbalance := NewImbalance(firstCandle.Market, high, midpoint, low, sentiment, gapRatio, thirdCandle.Date)

		return imbalance, true
	}

	return nil, false
}
