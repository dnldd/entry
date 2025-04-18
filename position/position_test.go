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
	entrySignal := &shared.EntrySignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     10,
		Reasons:   []shared.Reason{shared.BullishEngulfing, shared.StrongVolume},
		StopLoss:  8,
	}

	// Ensure positions cannot be created with nil entry signals.
	position, err := NewPosition(nil)
	assert.Error(t, err)

	// Ensure positions can be created with valid entry signals.
	position, err = NewPosition(entrySignal)
	assert.NoError(t, err)

	// Ensure position's profit and loss can be updated.
	currentPrice := float64(15)
	position.UpdatePNLPercent(currentPrice)
	assert.GreaterThan(t, position.PNLPercent, 0)

	// Ensure a position can be closed.
	exitSignal := &shared.ExitSignal{
		Market:    "^GSPC",
		Timeframe: shared.FiveMinute,
		Direction: shared.Long,
		Price:     18,
		Reasons:   []shared.Reason{shared.TargetHit},
	}

	status, err := position.ClosePosition(exitSignal)
	assert.NoError(t, err)
	assert.Equal(t, status, Closed)
}
