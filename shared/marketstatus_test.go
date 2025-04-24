package shared

import "testing"

func TestMarketStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status MarketStatus
		want   string
	}{
		{
			"neutral inclination",
			NeutralInclination,
			"neutral inclination",
		},
		{
			"long inclined",
			LongInclined,
			"long inclined",
		},
		{
			"short inclined",
			ShortInclined,
			"short inclined",
		},
		{
			"unknown",
			MarketStatus(999),
			"unknown",
		},
	}

	for _, test := range tests {
		str := test.status.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
