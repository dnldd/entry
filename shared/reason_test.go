package shared

import "testing"

func TestEntryReasonString(t *testing.T) {
	tests := []struct {
		name   string
		reason Reason
		want   string
	}{
		{
			"target hit",
			TargetHit,
			"target hit",
		},
		{
			"bullish engulfing",
			BullishEngulfing,
			"bullish engulfing",
		},
		{
			"bearish engulfing",
			BearishEngulfing,
			"bearish engulfing",
		},
		{
			"price reversal at support",
			ReversalAtSupport,
			"price reversal at support",
		},
		{
			"price reversal at resistance",
			ReversalAtResistance,
			"price reversal at resistance",
		},
		{
			"strong volume",
			StrongVolume,
			"strong volume",
		},
		{
			"unknown reason",
			Reason(999),
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
