package shared

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/peterldowns/testy/assert"
)

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

func TestLevel(t *testing.T) {
	price := float64(12)
	market := "^GSPC"
	firstCandle := &Candlestick{
		Open:  10,
		High:  15,
		Low:   9,
		Close: 5,
	}

	// Ensure a level can be initialized.
	lvl := NewLevel(market, price, firstCandle)
	assert.Equal(t, lvl.Kind, Resistance)

	// Ensure a level can be updated.
	reversalReaction := Reversal
	lvl.ApplyReaction(reversalReaction)
	assert.Equal(t, lvl.Reversals.Load(), uint32(1))

	breakReaction := Break
	lvl.ApplyReaction(breakReaction)
	assert.True(t, lvl.Breaking.Load())

	// Ensure a level can be invalidated.
	secondCandle := &Candlestick{
		Open:  10,
		High:  15,
		Low:   9,
		Close: 13,
	}
	lvl.Update(secondCandle)
	assert.Equal(t, lvl.Breaks.Load(), uint32(1))
	assert.Equal(t, lvl.Kind, Support)

	lvl.ApplyReaction(breakReaction)
	assert.True(t, lvl.Breaking.Load())

	lvl.Update(firstCandle)
	assert.Equal(t, lvl.Breaks.Load(), uint32(2))
	assert.Equal(t, lvl.Kind, Resistance)

	lvl.ApplyReaction(breakReaction)
	assert.True(t, lvl.Breaking.Load())

	lvl.Update(secondCandle)
	assert.Equal(t, lvl.Breaks.Load(), uint32(3))
	assert.Equal(t, lvl.Kind, Support)

	assert.True(t, lvl.IsInvalidated())
}

func TestNewLevelReaction(t *testing.T) {
	price := float64(12)
	market := "^GSPC"
	resistanceCandle := &Candlestick{
		Open:  10,
		High:  15,
		Low:   9,
		Close: 5,
	}

	supportCandle := &Candlestick{
		Open:  13,
		High:  18,
		Low:   12,
		Close: 17,
	}

	tests := []struct {
		name              string
		level             *Level
		data              []*Candlestick
		wantReaction      Reaction
		wantPriceMovement []Movement
		wantErr           bool
	}{
		{
			name:  "insufficient data",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  10,
					High:  11,
					Low:   9,
					Close: 9,
				},
				{
					Open:  9,
					High:  10,
					Low:   8,
					Close: 8,
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: nil,
			wantErr:           true,
		},
		{
			name:  "reversal at resistance",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  10,
					High:  13,
					Low:   9,
					Close: 11,
				},
				{
					Open:  11,
					High:  14,
					Low:   10,
					Close: 10,
				},
				{
					Open:  10,
					High:  13,
					Low:   9,
					Close: 9,
				},
				{
					Open:  9,
					High:  12,
					Low:   8,
					Close: 8,
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []Movement{Below, Below, Below, Below},
			wantErr:           false,
		},
		{
			name:  "reversal at support",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  15,
					High:  16,
					Low:   10,
					Close: 14,
				},
				{
					Open:  10,
					High:  14,
					Low:   9,
					Close: 14,
				},
				{
					Open:  13,
					High:  14,
					Low:   13,
					Close: 14,
				},
				{
					Open:  14,
					High:  16,
					Low:   14,
					Close: 15,
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []Movement{Above, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:  "break at resistance",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  10,
					High:  13,
					Low:   9,
					Close: 11,
				},
				{
					Open:  11,
					High:  14,
					Low:   10,
					Close: 13,
				},
				{
					Open:  13,
					High:  15,
					Low:   12,
					Close: 14,
				},
				{
					Open:  14,
					High:  16,
					Low:   13,
					Close: 15,
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []Movement{Below, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:  "break at support",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  15,
					High:  16,
					Low:   12,
					Close: 13,
				},
				{
					Open:  13,
					High:  14,
					Low:   9,
					Close: 10,
				},
				{
					Open:  10,
					High:  12,
					Low:   8,
					Close: 9,
				},
				{
					Open:  9,
					High:  10,
					Low:   7,
					Close: 8,
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []Movement{Above, Below, Below, Below},
			wantErr:           false,
		},
		{
			name:  "chop reaction at support",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  11,
					High:  14,
					Low:   10,
					Close: 13,
				},
				{
					Open:  10,
					High:  13,
					Low:   9,
					Close: 11,
				},
				{
					Open:  11,
					High:  13,
					Low:   10,
					Close: 13,
				},
				{
					Open:  12,
					High:  15,
					Low:   11,
					Close: 11,
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []Movement{Above, Below, Above, Below},
			wantErr:           false,
		},
		{
			name:  "reversal at support - price consistently above level",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  20,
					High:  22,
					Low:   19,
					Close: 21,
				},
				{
					Open:  21,
					High:  25,
					Low:   22,
					Close: 23,
				},
				{
					Open:  23,
					High:  27,
					Low:   22,
					Close: 25,
				},
				{
					Open:  25,
					High:  30,
					Low:   25,
					Close: 28,
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []Movement{Above, Above, Above, Above},
			wantErr:           false,
		},
		{
			name:  "break at support - sharp price reversal to break support",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  20,
					High:  22,
					Low:   19,
					Close: 21,
				},
				{
					Open:  21,
					High:  25,
					Low:   22,
					Close: 23,
				},
				{
					Open:  23,
					High:  27,
					Low:   22,
					Close: 25,
				},
				{
					Open:  25,
					High:  40,
					Low:   10,
					Close: 11,
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []Movement{Above, Above, Above, Below},
			wantErr:           false,
		},
		{
			name:  "reversal at support - level rejection",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  15,
					High:  18,
					Low:   13,
					Close: 14,
				},
				{
					Open:  14,
					High:  14,
					Low:   9,
					Close: 10,
				},
				{
					Open:  10,
					High:  18,
					Low:   9,
					Close: 17,
				},
				{
					Open:  17,
					High:  20,
					Low:   16,
					Close: 19,
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []Movement{Above, Below, Above, Above},
			wantErr:           false,
		},
		{
			name:  "chop reaction at support - stagnant price",
			level: NewLevel(market, price, supportCandle),
			data: []*Candlestick{
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []Movement{Equal, Equal, Equal, Equal},
			wantErr:           false,
		},
		{
			name:  "chop reaction at resistance - stagnant price",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
				{
					Open:  12,
					High:  12,
					Low:   12,
					Close: 12,
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []Movement{Equal, Equal, Equal, Equal},
			wantErr:           false,
		},
		{
			name:  "break at resistance - sharp price reversal to break support",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  12,
					High:  13,
					Low:   9,
					Close: 10,
				},
				{
					Open:  10,
					High:  10,
					Low:   10,
					Close: 10,
				},
				{
					Open:  10,
					High:  12,
					Low:   8,
					Close: 9,
				},
				{
					Open:  9,
					High:  20,
					Low:   9,
					Close: 18,
				},
			},
			wantReaction:      Break,
			wantPriceMovement: []Movement{Below, Below, Below, Above},
			wantErr:           false,
		},
		{
			name:  "reversal at resistance - level rejection",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  15,
					High:  18,
					Low:   9,
					Close: 10,
				},
				{
					Open:  10,
					High:  15,
					Low:   9,
					Close: 14,
				},
				{
					Open:  14,
					High:  16,
					Low:   14,
					Close: 15,
				},
				{
					Open:  15,
					High:  16,
					Low:   8,
					Close: 10,
				},
			},
			wantReaction:      Reversal,
			wantPriceMovement: []Movement{Below, Above, Above, Below},
			wantErr:           false,
		},
		{
			name:  "chop reaction at resistance",
			level: NewLevel(market, price, resistanceCandle),
			data: []*Candlestick{
				{
					Open:  10,
					High:  13,
					Low:   9,
					Close: 11,
				},
				{
					Open:  11,
					High:  14,
					Low:   10,
					Close: 13,
				},
				{
					Open:  12,
					High:  15,
					Low:   11,
					Close: 11,
				},
				{
					Open:  11,
					High:  13,
					Low:   10,
					Close: 13,
				},
			},
			wantReaction:      Chop,
			wantPriceMovement: []Movement{Below, Above, Below, Above},
			wantErr:           false,
		},
		{
			name:    "not enough data",
			level:   NewLevel(market, price, supportCandle),
			data:    []*Candlestick{},
			wantErr: true,
		},
	}

	for _, test := range tests {
		reaction, err := NewLevelReaction(market, test.level, test.data)
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
