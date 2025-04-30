package shared

import "testing"

func TestMarketSkewString(t *testing.T) {
	tests := []struct {
		name string
		skew MarketSkew
		want string
	}{
		{
			"neutral skew",
			NeutralSkew,
			"neutral skew",
		},
		{
			"long skewed",
			LongSkewed,
			"long skewed",
		},
		{
			"short skewed",
			ShortSkewed,
			"short skewed",
		},
		{
			"unknown",
			MarketSkew(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.skew.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
