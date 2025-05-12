package shared

import "testing"

func TestPriceMovementString(t *testing.T) {
	tests := []struct {
		name     string
		movement PriceMovement
		want     string
	}{
		{
			"above price movement",
			Above,
			"above",
		},
		{
			"below price movement",
			Below,
			"below",
		},
		{
			"equal price movement",
			Equal,
			"equal",
		},
		{
			"unknown price movement",
			PriceMovement(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.movement.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestPriceReactionString(t *testing.T) {
	tests := []struct {
		name     string
		reaction PriceReaction
		want     string
	}{
		{
			"chop price reaction",
			Chop,
			"chop",
		},
		{
			"reversal price reaction",
			Reversal,
			"reversal",
		},
		{
			"break price reaction",
			Break,
			"break",
		},
		{
			"unknown price reaction",
			PriceReaction(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.reaction.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
