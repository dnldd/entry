package position

import (
	"strings"
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestPositionStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status PositionStatus
		want   string
	}{
		{
			name:   "active",
			status: Active,
			want:   "active",
		},
		{
			name:   "stopped out",
			status: StoppedOut,
			want:   "stopped out",
		},
		{
			name:   "closed",
			status: Closed,
			want:   "closed",
		},
		{
			name:   "unknown",
			status: PositionStatus(999),
			want:   "unknown",
		},
	}

	for _, test := range tests {
		str := test.status.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestStringifyEntryReasons(t *testing.T) {
	reasons := []shared.Reason{shared.TargetHit, shared.BullishEngulfing, shared.BearishEngulfing,
		shared.ReversalAtSupport, shared.ReversalAtResistance, shared.StrongVolume, shared.Reason(999)}

	str := stringifyReasons(reasons)
	assert.True(t, strings.Contains(str, "target hit"))
	assert.True(t, strings.Contains(str, "bullish engulfing"))
	assert.True(t, strings.Contains(str, "bearish engulfing"))
	assert.True(t, strings.Contains(str, "reversal at support"))
	assert.True(t, strings.Contains(str, "reversal at resistance"))
	assert.True(t, strings.Contains(str, "strong volume"))
	assert.True(t, strings.Contains(str, "unknown"))
}

func TestPosition(t *testing.T) {
	longEntrySignal := &shared.EntrySignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
		Status:    make(chan shared.StatusCode, 1),
	}

	invalidDirectionEntrySignal := &shared.EntrySignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Direction(999),
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
		Status:    make(chan shared.StatusCode, 1),
	}

	// Ensure creating a position errors if the direction of the entry is unknown.
	_, err := NewPosition(invalidDirectionEntrySignal)
	assert.Error(t, err)

	// Ensure positions cannot be created with nil entry signals.
	position, err := NewPosition(nil)
	assert.Error(t, err)

	// Ensure positions can be created with valid entry signals.
	position, err = NewPosition(longEntrySignal)
	assert.NoError(t, err)

	// Ensure position's profit and loss can be updated.
	currentPrice := float64(15)
	position.UpdatePNLPercent(currentPrice)
	assert.GreaterThan(t, position.PNLPercent, 0)

	// Ensure a position can be closed.
	longExitSignal := &shared.ExitSignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     18,
		Reasons:   []shared.Reason{shared.TargetHit},
		Status:    make(chan shared.StatusCode, 1),
	}

	status, err := position.ClosePosition(longExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, status, Closed)

	// Ensure a short position can be stopped out.
	shortEntrySignal := &shared.EntrySignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Short,
		Price:     20,
		Reasons:   []shared.Reason{shared.BearishEngulfing, shared.StrongVolume},
		StopLoss:  22,
		Status:    make(chan shared.StatusCode, 1),
	}

	position, err = NewPosition(shortEntrySignal)
	assert.NoError(t, err)

	currentPrice = float64(21)
	position.UpdatePNLPercent(currentPrice)
	assert.LessThan(t, position.PNLPercent, 0)

	shortExitSignal := &shared.ExitSignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     22,
		Reasons:   []shared.Reason{shared.StopLossHit},
		Status:    make(chan shared.StatusCode, 1),
	}

	status, err = position.ClosePosition(shortExitSignal)
	assert.NoError(t, err)
	assert.Equal(t, status, StoppedOut)

	// Ensure a long position can be stopped out.
	position, err = NewPosition(longEntrySignal)
	assert.NoError(t, err)

	currentPrice = float64(9)
	position.UpdatePNLPercent(currentPrice)
	assert.LessThan(t, position.PNLPercent, 0)

	exitSignal := &shared.ExitSignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     8,
		Reasons:   []shared.Reason{shared.StopLossHit},
		Status:    make(chan shared.StatusCode, 1),
	}

	status, err = position.ClosePosition(exitSignal)
	assert.NoError(t, err)
	assert.Equal(t, status, StoppedOut)
}
