package shared

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/peterldowns/testy/assert"
)

func TestFetchVWAPLevelKind(t *testing.T) {
	market := "^GSPC"
	now, _, _ := NewYorkTime()
	candle := Candlestick{
		Open:  float64(5),
		High:  float64(8),
		Low:   float64(3),
		Close: float64(6),
		Date:  now,

		Market:    market,
		Timeframe: FiveMinute,
		Status:    make(chan StatusCode, 1),
	}

	vwap := VWAP{
		Value: float64(5.5),
		Date:  now,
	}

	// Ensure the level kind of a vwap can be determined.
	levelKind := fetchVWAPLevelKind(&vwap, &candle)
	assert.Equal(t, levelKind, Support)
}

func TestNewReactionAtVWAP(t *testing.T) {
	market := "^GSPC"
	now, _, _ := NewYorkTime()

	tests := []struct {
		name     string
		candles  []*Candlestick
		vwapData []*VWAP
		wantErr  bool
		reaction ReactionAtFocus
	}{
		{
			"support vwap reversal",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(9),
					Low:   float64(5),
					Close: float64(8),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(8),
					High:  float64(12),
					Low:   float64(7),
					Close: float64(11),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(11),
					High:  float64(15),
					Low:   float64(10),
					Close: float64(14),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(4),
					Date:  now,
				},
				{
					Value: float64(4.1),
					Date:  now,
				},
				{
					Value: float64(4.2),
					Date:  now,
				},
				{
					Value: float64(4.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Support,
				CurrentPrice:  float64(14),
				Reaction:      Reversal,
				PriceMovement: []PriceMovement{Above, Above, Above, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"support vwap break",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(7),
					Low:   float64(3),
					Close: float64(4),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(4),
					High:  float64(5),
					Low:   float64(2),
					Close: float64(3),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(3),
					High:  float64(4),
					Low:   float64(0.5),
					Close: float64(1),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(4),
					Date:  now,
				},
				{
					Value: float64(4.1),
					Date:  now,
				},
				{
					Value: float64(4.2),
					Date:  now,
				},
				{
					Value: float64(4.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Support,
				CurrentPrice:  float64(1),
				Reaction:      Break,
				PriceMovement: []PriceMovement{Above, Below, Below, Below},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"support vwap sharp break",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(8),
					Low:   float64(5),
					Close: float64(5),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(5),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(7),
					Low:   float64(0.5),
					Close: float64(1),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(4),
					Date:  now,
				},
				{
					Value: float64(4.1),
					Date:  now,
				},
				{
					Value: float64(4.2),
					Date:  now,
				},
				{
					Value: float64(4.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Support,
				CurrentPrice:  float64(1),
				Reaction:      Break,
				PriceMovement: []PriceMovement{Above, Above, Above, Below},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"support vwap slow reversal",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(7),
					Low:   float64(2),
					Close: float64(3),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(3),
					High:  float64(5),
					Low:   float64(3),
					Close: float64(4.5),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(4.5),
					High:  float64(7),
					Low:   float64(3),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(4),
					Date:  now,
				},
				{
					Value: float64(4.1),
					Date:  now,
				},
				{
					Value: float64(4.2),
					Date:  now,
				},
				{
					Value: float64(4.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Support,
				CurrentPrice:  float64(6),
				Reaction:      Reversal,
				PriceMovement: []PriceMovement{Above, Below, Above, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"support vwap chop",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(7),
					Low:   float64(2),
					Close: float64(3),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(3),
					High:  float64(5),
					Low:   float64(3),
					Close: float64(4.5),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(4.5),
					High:  float64(5),
					Low:   float64(2),
					Close: float64(3),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(4),
					Date:  now,
				},
				{
					Value: float64(4.1),
					Date:  now,
				},
				{
					Value: float64(4.2),
					Date:  now,
				},
				{
					Value: float64(4.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Support,
				CurrentPrice:  float64(3),
				Reaction:      Chop,
				PriceMovement: []PriceMovement{Above, Below, Above, Below},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"resistance vwap reversal",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(9),
					Low:   float64(5),
					Close: float64(8),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(8),
					High:  float64(9),
					Low:   float64(6),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(8),
					Low:   float64(5),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(6),
				Reaction:      Reversal,
				PriceMovement: []PriceMovement{Below, Below, Below, Below},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"resistance vwap break",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(9),
					Low:   float64(5),
					Close: float64(8),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(8),
					High:  float64(12),
					Low:   float64(7),
					Close: float64(10),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(10),
					High:  float64(14),
					Low:   float64(9),
					Close: float64(13),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(13),
				Reaction:      Break,
				PriceMovement: []PriceMovement{Below, Below, Above, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"resistance vwap sharp break",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(9),
					Low:   float64(5),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(8),
					Low:   float64(5),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(14),
					Low:   float64(5),
					Close: float64(13),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(13),
				Reaction:      Break,
				PriceMovement: []PriceMovement{Below, Below, Below, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"resistance vwap slow reversal",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(9),
					Low:   float64(5),
					Close: float64(8),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(8),
					High:  float64(12),
					Low:   float64(7),
					Close: float64(10),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(10),
					High:  float64(11),
					Low:   float64(5),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(6),
				Reaction:      Reversal,
				PriceMovement: []PriceMovement{Below, Below, Above, Below},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"resistance vwap slow reversal",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(10),
					Low:   float64(5),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(9),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.4),
					Date:  now,
				},
			},
			false,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(9),
				Reaction:      Chop,
				PriceMovement: []PriceMovement{Below, Above, Below, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"empty vwap data",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(10),
					Low:   float64(5),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(9),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{},
			true,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(9),
				Reaction:      Chop,
				PriceMovement: []PriceMovement{Below, Above, Below, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"mismatched vwap data length",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(10),
					Low:   float64(5),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(9),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
			},
			true,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(9),
				Reaction:      Chop,
				PriceMovement: []PriceMovement{Below, Above, Below, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
		{
			"mismatched price data length",
			[]*Candlestick{
				{
					Open:  float64(5),
					High:  float64(7),
					Low:   float64(1),
					Close: float64(6),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(6),
					High:  float64(10),
					Low:   float64(5),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(9),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(7),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(7),
					High:  float64(12),
					Low:   float64(6),
					Close: float64(9),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
				{
					Open:  float64(9),
					High:  float64(15),
					Low:   float64(8),
					Close: float64(14),
					Date:  now,

					Market:    market,
					Timeframe: FiveMinute,
					Status:    make(chan StatusCode, 1),
				},
			},
			[]*VWAP{
				{
					Value: float64(8),
					Date:  now,
				},
				{
					Value: float64(8.1),
					Date:  now,
				},
				{
					Value: float64(8.2),
					Date:  now,
				},
				{
					Value: float64(8.3),
					Date:  now,
				},
			},
			true,
			ReactionAtFocus{
				Market:        market,
				Timeframe:     FiveMinute,
				LevelKind:     Resistance,
				CurrentPrice:  float64(9),
				Reaction:      Chop,
				PriceMovement: []PriceMovement{Below, Above, Below, Above},
				Status:        make(chan StatusCode, 1),
				CreatedOn:     now,
			},
		},
	}

	for _, test := range tests {
		reaction, err := NewReactionAtVWAP(market, test.vwapData, test.candles)
		if test.wantErr && err == nil {
			t.Errorf("%s: unexpected error, got %v", test.name, err)
		}

		if err == nil {
			if !cmp.Equal(reaction.ReactionAtFocus.Reaction, test.reaction.Reaction) {
				t.Errorf("%s: mismatching reaction, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.Reaction, test.reaction.Reaction))
			}
			if !cmp.Equal(reaction.ReactionAtFocus.PriceMovement, test.reaction.PriceMovement) {
				t.Errorf("%s: mismatching price movement, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.PriceMovement, test.reaction.PriceMovement))
			}
			if !cmp.Equal(reaction.ReactionAtFocus.Market, test.reaction.Market) {
				t.Errorf("%s: mismatching market, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.Market, test.reaction.Market))
			}
			if !cmp.Equal(reaction.ReactionAtFocus.Timeframe, test.reaction.Timeframe) {
				t.Errorf("%s: mismatching timeframe, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.Timeframe, test.reaction.Timeframe))
			}
			if !cmp.Equal(reaction.ReactionAtFocus.CurrentPrice, test.reaction.CurrentPrice) {
				t.Errorf("%s: mismatching current price, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.CurrentPrice, test.reaction.CurrentPrice))
			}
			if !cmp.Equal(reaction.ReactionAtFocus.CreatedOn, test.reaction.CreatedOn) {
				t.Errorf("%s: mismatching created on, got %v", test.name, cmp.Diff(reaction.ReactionAtFocus.CreatedOn, test.reaction.CreatedOn))
			}

			assert.NotNil(t, reaction.ReactionAtFocus.Status)
		}
	}
}
