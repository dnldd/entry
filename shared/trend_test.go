package shared

import "testing"

func TestTrendString(t *testing.T) {
	tests := []struct {
		name  string
		trend Trend
		want  string
	}{
		{
			"choppy trend",
			ChoppyTrend,
			"choppy trend",
		},

		{
			"mild bullish trend",
			MildBullishTrend,
			"mild bullish trend",
		},
		{
			"mild bearish trend",
			MildBearishTrend,
			"mild bearish trend",
		},
		{
			"strong bullish trend",
			StrongBullishTrend,
			"strong bullish trend",
		},
		{
			"strong bearish trend",
			StrongBearishTrend,
			"strong bearish trend",
		},
		{
			"unknown trend",
			Trend(999),
			"unknown trend",
		},
	}

	for _, test := range tests {
		str := test.trend.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
