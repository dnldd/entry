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
	entryReasons := []shared.EntryReason{shared.BullishEngulfingEntry, shared.BearishEngulfingEntry,
		shared.ReversalAtSupportEntry, shared.ReversalAtResistanceEntry, shared.StrongVolumeEntry, shared.EntryReason(999)}

	str := stringifyEntryReasons(entryReasons)
	assert.True(t, strings.Contains(str, "bullish engulfing"))
	assert.True(t, strings.Contains(str, "bearish engulfing"))
	assert.True(t, strings.Contains(str, "reversal at support"))
	assert.True(t, strings.Contains(str, "reversal at resistance"))
	assert.True(t, strings.Contains(str, "strong volume"))
	assert.True(t, strings.Contains(str, "unknown"))
}

func TestStringifyExitReasons(t *testing.T) {
	exitReasons := []shared.ExitReason{shared.TargetHitExit, shared.BullishEngulfingExit, shared.BearishEngulfingExit,
		shared.ReversalAtSupportExit, shared.ReversalAtResistanceExit, shared.StrongVolumeExit, shared.ExitReason(999)}

	str := stringifyExitReasons(exitReasons)
	assert.True(t, strings.Contains(str, "target hit"))
	assert.True(t, strings.Contains(str, "bullish engulfing"))
	assert.True(t, strings.Contains(str, "bearish engulfing"))
	assert.True(t, strings.Contains(str, "reversal at support"))
	assert.True(t, strings.Contains(str, "reversal at resistance"))
	assert.True(t, strings.Contains(str, "strong volume"))
	assert.True(t, strings.Contains(str, "unknown"))
}
