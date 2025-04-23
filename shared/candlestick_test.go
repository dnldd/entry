package shared

import "testing"

func TestFetchSentiment(t *testing.T) {
	tests := []struct {
		name   string
		candle Candlestick
		want   Sentiment
	}{
		{
			name: "neutral candle",
			candle: Candlestick{
				Open:  5,
				Close: 5,
				High:  9,
				Low:   1,
			},
			want: Neutral,
		},
		{
			name: "bullish candle",
			candle: Candlestick{
				Open:  5,
				Close: 15,
				High:  20,
				Low:   1,
			},
			want: Bullish,
		},
		{
			name: "bearish candle",
			candle: Candlestick{
				Open:  15,
				Close: 5,
				High:  20,
				Low:   1,
			},
			want: Bearish,
		},
	}

	for _, test := range tests {
		sentiment := test.candle.FetchSentiment()
		if sentiment != test.want {
			t.Errorf("%s: expected %s sentiment, got %s",
				test.name, sentiment.String(), test.want.String())
		}
	}
}

func TestFetchKind(t *testing.T) {
	tests := []struct {
		name   string
		candle Candlestick
		want   Kind
	}{
		{
			name: "doji",
			candle: Candlestick{
				Open:  5,
				Close: 5,
				High:  9,
				Low:   1,
			},
			want: Doji,
		},
		{
			name: "marubozu",
			candle: Candlestick{
				Open:  5,
				Close: 20,
				High:  21,
				Low:   4,
			},
			want: Marubozu,
		},
		{
			name: "pinbar (bullish)",
			candle: Candlestick{
				Open:  10,
				Close: 15,
				High:  17,
				Low:   1,
			},
			want: Pinbar,
		},
		{
			name: "pinbar (bearish)",
			candle: Candlestick{
				Open:  10,
				Close: 7,
				High:  17,
				Low:   6,
			},
			want: Pinbar,
		},
		{
			name: "unknown",
			candle: Candlestick{
				Open:  1,
				Close: 1,
				High:  1,
				Low:   1,
			},
			want: Unknown,
		},
		{
			name: "unknown",
			candle: Candlestick{
				Open:  0,
				Close: 0,
				High:  0,
				Low:   0,
			},
			want: Unknown,
		},
	}

	for _, test := range tests {
		kind := test.candle.FetchKind()
		if kind != test.want {
			t.Errorf("%s: expected %s kind, got %s",
				test.name, kind.String(), test.want.String())
		}
	}
}

func TestIsVolumeSpike(t *testing.T) {
	tests := []struct {
		name    string
		current *Candlestick
		prev    *Candlestick
		want    bool
	}{
		{
			name: "no volume spike (below threshold)",
			current: &Candlestick{
				Volume: 10,
			},
			prev: &Candlestick{
				Volume: 9,
			},
			want: false,
		},
		{
			name: "no volume spike (negative difference)",
			current: &Candlestick{
				Volume: 10,
			},
			prev: &Candlestick{
				Volume: 20,
			},
			want: false,
		},
		{
			name: "volume spike",
			current: &Candlestick{
				Volume: 10,
			},
			prev: &Candlestick{
				Volume: 5,
			},
			want: true,
		},
		{
			name: "no volume spike (no volume)",
			current: &Candlestick{
				Volume: 0,
			},
			prev: &Candlestick{
				Volume: 0,
			},
			want: false,
		},
	}

	for _, test := range tests {
		isVolumeSpike := IsVolumeSpike(test.current, test.prev)
		if isVolumeSpike != test.want {
			t.Errorf("%s: expected %v, got %v",
				test.name, test.want, isVolumeSpike)
		}
	}
}

func TestGenerateMomemtum(t *testing.T) {
	tests := []struct {
		name    string
		current *Candlestick
		prev    *Candlestick
		want    Momentum
	}{
		{
			name: "low momentum (zero volume)",
			current: &Candlestick{
				Volume: 0,
			},
			prev: &Candlestick{
				Volume: 0,
			},
			want: Low,
		},
		{
			name: "low momemtum",
			current: &Candlestick{
				Volume: 0.8,
			},
			prev: &Candlestick{
				Volume: 1,
			},
			want: Low,
		},
		{
			name: "medium momentum",
			current: &Candlestick{
				Volume: 10,
			},
			prev: &Candlestick{
				Volume: 8,
			},
			want: Medium,
		},
		{
			name: "high momentum",
			current: &Candlestick{
				Volume: 10,
			},
			prev: &Candlestick{
				Volume: 4,
			},
			want: High,
		},
	}

	for _, test := range tests {
		momentum := GenerateMomentum(test.current, test.prev)
		if momentum != test.want {
			t.Errorf("%s: expected %v, got %v",
				test.name, test.want.String(), momentum.String())
		}
	}
}

func TestIsEngulfing(t *testing.T) {
	tests := []struct {
		name    string
		current *Candlestick
		prev    *Candlestick
		want    bool
	}{
		{
			name: "not engulfing (prev is a doji)",
			current: &Candlestick{
				Open:  5,
				Close: 8,
				High:  10,
				Low:   4,
			},
			prev: &Candlestick{
				Open:  5,
				Close: 6,
				High:  10,
				Low:   1,
			},
			want: false,
		},
		{
			name: "not engulfing (two bullish candles)",
			current: &Candlestick{
				Open:  9,
				Close: 15,
				High:  18,
				Low:   6,
			},
			prev: &Candlestick{
				Open:  5,
				Close: 8,
				High:  10,
				Low:   4,
			},
			want: false,
		},
		{
			name: "engulfing (bullish)",
			current: &Candlestick{
				Open:  2,
				Close: 15,
				Low:   1,
				High:  16,
			},
			prev: &Candlestick{
				Open:  5,
				Close: 2,
				Low:   1,
				High:  10,
			},
			want: true,
		},
		{
			name: "engulfing (bearish)",
			current: &Candlestick{
				Open:  10,
				Close: 1,
				Low:   1,
				High:  11,
			},
			prev: &Candlestick{
				Open:  5,
				Close: 8,
				Low:   2,
				High:  9,
			},
			want: true,
		},
		{
			name: "not engulfing (weak bullish engulfing with long wick)",
			current: &Candlestick{
				Open:  5,
				Close: 8,
				Low:   1,
				High:  9,
			},
			prev: &Candlestick{
				Open:  7,
				Close: 5,
				Low:   4,
				High:  8,
			},
			want: false,
		},
	}

	for _, test := range tests {
		engulfing := IsEngulfing(test.current, test.prev)
		if engulfing != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, engulfing)
		}
	}
}

func TestMomentumString(t *testing.T) {
	tests := []struct {
		name     string
		momentum Momentum
		want     string
	}{
		{
			"high momentum",
			High,
			"high",
		},
		{
			"medium momentum",
			Medium,
			"medium",
		},
		{
			"low momentum",
			Low,
			"low",
		},
		{
			"unknown momentum",
			Momentum(999),
			"low",
		},
	}

	for _, test := range tests {
		str := test.momentum.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		want string
	}{
		{
			"marubozu kind",
			Marubozu,
			"marubozu",
		},
		{
			"pinbar kind",
			Pinbar,
			"pinbar",
		},
		{
			"doji kind",
			Doji,
			"doji",
		},
		{
			"unknown kind",
			Kind(999),
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

func TestSentimentString(t *testing.T) {
	tests := []struct {
		name      string
		sentiment Sentiment
		want      string
	}{
		{
			"neutral sentiment",
			Neutral,
			"neutral",
		},
		{
			"bullish sentiment",
			Bullish,
			"bullish",
		},
		{
			"bearish sentiment",
			Bearish,
			"bearish",
		},
		{
			"unknown sentiment",
			Sentiment(999),
			"neutral",
		},
	}

	for _, test := range tests {
		str := test.sentiment.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}
