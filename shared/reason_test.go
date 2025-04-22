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
			"price break below support",
			BreakBelowSupport,
			"price break below support",
		},
		{
			"price break above resistance",
			BreakAboveResistance,
			"price break above resistance",
		},
		{
			"strong volume",
			StrongVolume,
			"strong volume",
		},
		{
			"strong move",
			StrongMove,
			"strong move",
		}, {
			"high volume session",
			HighVolumeSession,
			"high volume session",
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
