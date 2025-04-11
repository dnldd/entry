package shared

import "testing"

func TestMovementString(t *testing.T) {
	tests := []struct {
		name     string
		movement Movement
		want     string
	}{
		{
			"above movement",
			Above,
			"above",
		},
		{
			"below movement",
			Below,
			"below",
		},
		{
			"unknown movement",
			Movement(999),
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

func TestReactionString(t *testing.T) {
	tests := []struct {
		name     string
		reaction Reaction
		want     string
	}{
		{
			"chop reaction",
			Chop,
			"chop",
		},
		{
			"reversal reaction",
			Reversal,
			"reversal",
		},
		{
			"break reaction",
			Break,
			"break",
		},
		{
			"unknown reaction",
			Reaction(999),
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
