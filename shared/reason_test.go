package shared

import "testing"

func TestEntryReasonString(t *testing.T) {
	tests := []struct {
		name   string
		reason EntryReason
		want   string
	}{
		{
			"bullish engulfing",
			BullishEngulfingEntry,
			"bullish engulfing",
		},
		{
			"bearish engulfing",
			BearishEngulfingEntry,
			"bearish engulfing",
		},
		{
			"price reversal at support",
			ReversalAtSupportEntry,
			"price reversal at support",
		},
		{
			"price reversal at resistance",
			ReversalAtResistanceEntry,
			"price reversal at resistance",
		},
		{
			"strong volume",
			StrongVolumeEntry,
			"strong volume",
		},
		{
			"unknown entry reason",
			EntryReason(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.reason.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestExitReasonString(t *testing.T) {
	tests := []struct {
		name   string
		reason ExitReason
		want   string
	}{
		{
			"target hit",
			TargetHitExit,
			"target hit",
		},
		{
			"bullish engulfing",
			BullishEngulfingExit,
			"bullish engulfing",
		},
		{
			"bearish engulfing",
			BearishEngulfingExit,
			"bearish engulfing",
		},
		{
			"price reversal at support",
			ReversalAtSupportExit,
			"price reversal at support",
		},
		{
			"price reversal at resistance",
			ReversalAtResistanceExit,
			"price reversal at resistance",
		},
		{
			"strong volume",
			StrongVolumeExit,
			"strong volume",
		},
		{
			"unknown exit reason",
			ExitReason(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.reason.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestDirectionString(t *testing.T) {
	tests := []struct {
		name      string
		direction Direction
		want      string
	}{
		{
			"long direction",
			Long,
			"long",
		},
		{
			"short direction",
			Short,
			"short",
		},
		{
			"unknown direction",
			Direction(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.direction.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
