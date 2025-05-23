package shared

import (
	"errors"
	"math"
	"sync"
	"time"

	"go.uber.org/atomic"
)

const (
	// MacroTrend is the total number of 5-minute candles used to create a high timeframe trend perpective.
	MacroTrend = 60
	// LocalTrend is the total number of 5-minute candles used to create a low timeframe trend perspective.
	LocalTrend = 20
	// minStrongR2 is the mimimum r2 value considered a strong fit.
	minStrongR2 = 0.80
	// minStrongSlope is the mimimum score a score needs to be considered strong.
	minStrongSlope = 0.5
)

// VWAPSnapshot represents a snapshot of vwap data.
type VWAPSnapshot struct {
	data      []*VWAP
	dataMtx   sync.RWMutex
	timeframe Timeframe
	start     atomic.Int32
	count     atomic.Int32
	size      atomic.Int32
}

// NewVWAPSnapshot initializes a new vwap snapshot.
func NewVWAPSnapshot(size int32, timeframe Timeframe) (*VWAPSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &VWAPSnapshot{
		timeframe: timeframe,
		data:      make([]*VWAP, size),
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
	if n <= 0 {
		return nil
	}

	s.dataMtx.RLock()
	defer s.dataMtx.RUnlock()

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

// categorizeTrend classifies the trend based on the slope fit.
func categorizeTrend(slope float64, r2 float64) Trend {
	var trend Trend
	absSlope := math.Abs(slope)

	// If the slope is twice as weak as the minimum to be considered strong then the trend is choppy.
	if absSlope < minStrongSlope/2 {
		trend = ChoppyTrend
	}

	switch {
	// If r2 and the slope are both below the minimum to be considered strong then the trend is mild one.
	case r2 < minStrongR2 && absSlope < minStrongSlope:
		switch {
		case slope > 0:
			trend = MildBullishTrend
		case slope < 0:
			trend = MildBearishTrend
		default:
			trend = ChoppyTrend
		}

	// If r2 and the slope are above the minimum to be considered strong then for a linear
	// progressing scenario it is a strong trend. For a parabolic scenario r2 expectedly
	// will be low but the slope will be magnitudes higher for a strong trend.
	case (r2 > minStrongR2 && absSlope > minStrongSlope) || (r2 < minStrongR2 && r2 > minStrongR2/2 && absSlope > minStrongSlope*2):
		switch {
		case slope > 0:
			trend = StrongBullishTrend

		case slope < 0:
			trend = StrongBearishTrend
		default:
			trend = ChoppyTrend
		}
	}

	return trend
}

// TrendScore determines the strength of the market trend from the vwap snapshot generated from it.
//
// This uses linear regression slope and qualifies it with r squared to generate the trend.
func (s *VWAPSnapshot) Trend(n int32) (Trend, float64, float64) {
	if n <= 0 {
		return ChoppyTrend, 0, 0
	}

	values := s.LastN(n)
	nf := float64(n)

	// Calculate the linear regression slope of the vwap which is the strength of the trend.
	// The slope can either be positive or negative. A high slope value regardless of sign
	// indicates a strong trend while being closest to zero indicate chop.
	var sumX, sumY, sumXY, sumXX float64
	for idx, y := range values {
		x := float64(idx)
		sumX += x
		sumY += y.Value
		sumXY += x * y.Value
		sumXX += x * x
	}

	numerator := (nf * sumXY) - (sumX * sumY)
	denominator := (nf * sumXX) - (sumX * sumX)
	if denominator == 0 {
		return ChoppyTrend, 0, 0
	}

	slope := numerator / denominator
	meanY := sumY / float64(n)

	// Calculate total sum of squares and residual sum of squares.
	var totalSum, residualSum float64
	intercept := meanY - slope*(sumX/nf)
	for idx, v := range values {
		x := float64(idx)
		y := v.Value
		yPred := slope*x + intercept
		totalSum += (y - meanY) * (y - meanY)
		residualSum += (y - yPred) * (y - yPred)
	}

	if totalSum == 0 {
		return ChoppyTrend, 0, 0
	}

	// Calculate r2 (r squared) which is the confidence metric of the linear regression slope.
	r2 := min(max(1-(residualSum/totalSum), 0), 1)

	return categorizeTrend(slope, r2), slope, r2
}
