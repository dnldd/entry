package shared

import "testing"

func TestLevelKindString(t *testing.T) {
	tests := []struct {
		name string
		kind LevelKind
		want string
	}{
		{
			"support level",
			Support,
			"support",
		},
		{
			"resistance level",
			Resistance,
			"resistance",
		},
		{
			"unknown level kind",
			LevelKind(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.kind.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
