package shared

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/peterldowns/testy/assert"
)

func TestImbalanceUpdate(t *testing.T) {
	// Ensure an imbalance can be created.
	market := "^GSPC"
	timeframe := FiveMinute
	high := float64(23)
	midpoint := float64(20.5)
	low := float64(18)
	gapRatio := float64(0.7142857142857143)
	bullishImbalance := NewImbalance(market, timeframe, high, midpoint, low, Bullish, gapRatio, time.Time{})
	bearishImbalance := NewImbalance(market, timeframe, low, midpoint, high, Bearish, gapRatio, time.Time{})

	// Ensure an imbalance can be updated by new candlestick data.
	bullishPurgeCandle := &Candlestick{
		Market:    market,
		Open:      float64(25),
		Close:     float64(16),
		High:      float64(26),
		Low:       float64(14),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	// Ensure an imbalance can be updated by new candlestick data.
	bearishPurgeCandle := &Candlestick{
		Market:    market,
		Open:      float64(14),
		Close:     float64(30),
		High:      float64(32),
		Low:       float64(13),
		Volume:    float64(2),
		Status:    make(chan StatusCode, 1),
		Timeframe: timeframe,
	}

	bullishImbalance.Update(bullishPurgeCandle)
	assert.True(t, bullishImbalance.Purged.Load())

	bearishImbalance.Update(bearishPurgeCandle)
	assert.True(t, bullishImbalance.Purged.Load())

	// Ensure a subsequente close beyond the imbalance invalidates it.
	bullishImbalance.Update(bullishPurgeCandle)
	assert.True(t, bullishImbalance.Invalidated.Load())

	bearishImbalance.Update(bearishPurgeCandle)
	assert.True(t, bullishImbalance.Invalidated.Load())
}

func TestNewReactionAtImbalance(t *testing.T) {
	price := float64(12)
	market := "^GSPC"
	timeframe := FiveMinute
	gapRatio := float64(0.75)
	bullishImbalance := NewImbalance(market, timeframe, price+4, price+2, price, Bullish, gapRatio, time.Time{})
	bearishImbalance := NewImbalance(market, timeframe, price, price-2, price-4, Bearish, gapRatio, time.Time{})

	tests := []struct {
		name              string
		imbalance         *Imbalance
		data              []*Candlestick
		wantReaction      PriceReaction
		wantPriceMovement []PriceMovement
		wantErr           bool
	}{
		{
			name:      "insufficient data",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   10,
					High:   11,
					Low:    9,
					Close:  9,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   9,
					High:   10,
					Low:    8,
					Close:  8,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: nil,
			wantErr:           true,
		},
		{
			name:      "reversal at resistance",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   10,
					High:   13,
					Low:    9,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   11,
					High:   14,
					Low:    10,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   13,
					Low:    9,
					Close:  9,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   9,
					High:   12,
					Low:    8,
					Close:  8,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []PriceMovement{Below, Below, Below, Below},
			wantErr:           false,
		},
		{
			name:      "reversal at support",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   15,
					High:   16,
					Low:    10,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   14,
					Low:    9,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   13,
					High:   14,
					Low:    13,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   14,
					High:   16,
					Low:    14,
					Close:  15,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []PriceMovement{Above, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:      "break at resistance",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   10,
					High:   13,
					Low:    9,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   11,
					High:   14,
					Low:    10,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   13,
					High:   15,
					Low:    12,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   14,
					High:   16,
					Low:    13,
					Close:  15,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []PriceMovement{Below, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:      "break at support",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   15,
					High:   16,
					Low:    12,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   13,
					High:   14,
					Low:    9,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   12,
					Low:    8,
					Close:  9,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   9,
					High:   10,
					Low:    7,
					Close:  8,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []PriceMovement{Above, Below, Below, Below},
			wantErr:           false,
		},
		{
			name:      "chop reaction at support",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   11,
					High:   14,
					Low:    10,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   13,
					Low:    9,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   11,
					High:   13,
					Low:    10,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   15,
					Low:    11,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []PriceMovement{Above, Below, Above, Below},
			wantErr:           false,
		},
		{
			name:      "reversal at support - price consistently above imbalance",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   20,
					High:   22,
					Low:    19,
					Close:  21,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   21,
					High:   25,
					Low:    22,
					Close:  23,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   23,
					High:   27,
					Low:    22,
					Close:  25,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   25,
					High:   30,
					Low:    25,
					Close:  28,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []PriceMovement{Above, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:      "break at support - sharp price reversal to break support",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   20,
					High:   22,
					Low:    19,
					Close:  21,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   21,
					High:   25,
					Low:    22,
					Close:  23,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   23,
					High:   27,
					Low:    22,
					Close:  25,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   25,
					High:   40,
					Low:    10,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []PriceMovement{Above, Above, Above, Below},
			wantErr:           false,
		},
		{
			name:      "reversal at support - imbalance rejection",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   15,
					High:   18,
					Low:    13,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   14,
					High:   14,
					Low:    9,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   18,
					Low:    9,
					Close:  17,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   17,
					High:   20,
					Low:    16,
					Close:  19,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []PriceMovement{Above, Below, Above, Above},
			wantErr:           false,
		},
		{
			name:      "chop reaction at support - stagnant price",
			imbalance: bullishImbalance,
			data: []*Candlestick{
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []PriceMovement{Equal, Equal, Equal, Equal},
			wantErr:           false,
		},
		{
			name:      "chop reaction at resistance - stagnant price",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   12,
					Low:    12,
					Close:  12,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []PriceMovement{Equal, Equal, Equal, Equal},
			wantErr:           false,
		},
		{
			name:      "break at resistance - sharp price reversal to break support",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   12,
					High:   13,
					Low:    9,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   10,
					Low:    10,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   12,
					Low:    8,
					Close:  9,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   9,
					High:   20,
					Low:    9,
					Close:  18,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []PriceMovement{Below, Below, Below, Above},
			wantErr:           false,
		},
		{
			name:      "reversal at resistance - imbalance rejection",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   15,
					High:   18,
					Low:    9,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   10,
					High:   15,
					Low:    9,
					Close:  14,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   14,
					High:   16,
					Low:    14,
					Close:  15,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   15,
					High:   16,
					Low:    8,
					Close:  10,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []PriceMovement{Below, Above, Above, Below},
			wantErr:           false,
		},
		{
			name:      "chop reaction at resistance",
			imbalance: bearishImbalance,
			data: []*Candlestick{
				{
					Open:   10,
					High:   13,
					Low:    9,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   11,
					High:   14,
					Low:    10,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   12,
					High:   15,
					Low:    11,
					Close:  11,
					Status: make(chan StatusCode, 1),
				},
				{
					Open:   11,
					High:   13,
					Low:    10,
					Close:  13,
					Status: make(chan StatusCode, 1),
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []PriceMovement{Below, Above, Below, Above},
			wantErr:           false,
		},
		{
			name:      "not enough data",
			imbalance: bullishImbalance,
			data:      []*Candlestick{},
			wantErr:   true,
		},
	}

	for _, test := range tests {
		reaction, err := NewReactionAtImbalance(market, test.imbalance, test.data)
		if err == nil && test.wantErr {
			t.Errorf("%s: expected an error, got none", test.name)
		}

		if err != nil && !test.wantErr {
			t.Errorf("%s: no error expected but got %v", test.name, err)
		}

		if err == nil {
			if !cmp.Equal(test.wantPriceMovement, reaction.PriceMovement) {
				t.Errorf("%s: expected movement %v, got %v", test.name, test.wantPriceMovement, reaction.PriceMovement)
			}
			if reaction.Reaction != test.wantReaction {
				t.Errorf("%s: expected reaction %v, got %v", test.name, test.wantReaction.String(), reaction.Reaction.String())
			}
		}
	}
}
