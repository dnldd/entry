package shared

import (
	"fmt"
	"time"

	"go.uber.org/atomic"
)

// Imbalance represents market inefficiencies created by displacement. These act as high
// probability reaction levels for price.
type Imbalance struct {
	Market      string
	High        float64
	Midpoint    float64
	Low         float64
	Timeframe   Timeframe
	Sentiment   Sentiment
	GapRatio    float64
	Purged      atomic.Bool
	Invalidated atomic.Bool
	Date        time.Time
}

// NewImbalance initializes a new imbalance.
func NewImbalance(market string, timeframe Timeframe, high float64, midpoint float64, low float64,
	sentiment Sentiment, gapRatio float64, date time.Time) *Imbalance {
	return &Imbalance{
		Market:    market,
		High:      high,
		Midpoint:  midpoint,
		Low:       low,
		Timeframe: timeframe,
		Sentiment: sentiment,
		GapRatio:  gapRatio,
		Date:      date,
	}
}

// Update updates the imbalance with the provided candstick.
func (imb *Imbalance) Update(candle *Candlestick) {
	purged := imb.Purged.Load()
	invalidated := imb.Invalidated.Load()

	if invalidated {
		return
	}

	switch imb.Sentiment {
	case Bullish:
		// If the imbalance is bullish then price closing below the low
		// of the imbalance range twice invalidates it.
		switch {
		case candle.Close < imb.Low && !purged:
			imb.Purged.Store(true)

		case candle.Close < imb.Low && purged:
			imb.Invalidated.Store(true)
		}

	case Bearish:
		// If the imbalance is bearish then price closing above the high
		// of the imbalance range twice invalidates it.
		switch {
		case candle.Close > imb.High && !purged:
			imb.Purged.Store(true)

		case candle.Close > imb.High && purged:
			imb.Invalidated.Store(true)
		}
	}
}

// ReactionAtImbalance describes the reaction of price relative to an imabalance.
type ReactionAtImbalance struct {
	ReactionAtFocus
	Imbalance *Imbalance
}

// NewReactionAtImbalance initializes a new reaction from the provided imbalance and candlestick data.
func NewReactionAtImbalance(market string, imbalance *Imbalance, priceData []*Candlestick) (*ReactionAtImbalance, error) {
	if len(priceData) != PriceDataPayloadSize {
		return nil, fmt.Errorf("price data is not the expected size: %d != expected(%d)",
			len(priceData), PriceDataPayloadSize)
	}

	var levelKind LevelKind
	switch imbalance.Sentiment {
	case Bullish:
		levelKind = Support
	case Bearish:
		levelKind = Resistance
	}

	ir := &ReactionAtImbalance{
		ReactionAtFocus: ReactionAtFocus{
			Market:        market,
			LevelKind:     levelKind,
			Timeframe:     priceData[len(priceData)-1].Timeframe,
			PriceMovement: make([]PriceMovement, 0, len(priceData)),
			Status:        make(chan StatusCode, 1),
			CurrentPrice:  priceData[len(priceData)-1].Close,
			CreatedOn:     priceData[len(priceData)-1].Date,
		},
		Imbalance: imbalance,
	}

	// Generate price movement data from the level and provided price data.
	for idx := range priceData {
		candle := priceData[idx]

		switch levelKind {
		case Support:
			// Support imbalances will use the lowest point of their range as the level that has to
			// be broken to be invalidated.
			switch {
			case candle.Close > imbalance.Low:
				ir.PriceMovement = append(ir.PriceMovement, Above)
			case candle.Close < imbalance.Low:
				ir.PriceMovement = append(ir.PriceMovement, Below)
			case candle.Close == imbalance.Low:
				ir.PriceMovement = append(ir.PriceMovement, Equal)
			}

		case Resistance:
			// Resistance imbalances will use the highest point of their range as the level that has to
			// be broken to be invalidated.
			switch {
			case candle.Close < imbalance.High:
				ir.PriceMovement = append(ir.PriceMovement, Below)
			case candle.Close > imbalance.High:
				ir.PriceMovement = append(ir.PriceMovement, Above)
			case candle.Close == imbalance.High:
				ir.PriceMovement = append(ir.PriceMovement, Equal)
			}
		}
	}

	// Generate a price reaction based on the price movement data.
	var above, below uint32
	for idx := range ir.PriceMovement {
		switch {
		case ir.PriceMovement[idx] == Above:
			above++
		case ir.PriceMovement[idx] == Below:
			below++
		}
	}

	// The imbalance reaction is currently rooted in being able to make a decision
	// on a reaction using 4 5-minute candles. Changing the data size would
	// require reworking the logic here.

	first := ir.PriceMovement[0]
	second := ir.PriceMovement[1]
	third := ir.PriceMovement[2]
	fourth := ir.PriceMovement[3]

	switch levelKind {
	case Support:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the imbalance it is chopping.
			ir.Reaction = Chop
		case below == 0:
			// If price consistently stayed below a support imbalance it tagged then it
			// it is likely reversing at the vwap.
			ir.Reaction = Reversal
		case first == Above && third == Below && fourth == Below:
			// If price was above an imbalance acting as support but starts to consistently close below it
			// then it is likely breaking the imbalance.
			ir.Reaction = Break
		case first == Above && second == Above && third == Above && fourth == Below:
			// If price was above an imbalance acting as support but turns sharply to close below it then
			// it is likely breaking the imbalance.
			ir.Reaction = Break
		case first == Above && below > 0 && fourth == Above:
			// If price was above an imbalance acting as support level but closed below it briefly and
			// pushed back above it then it is likely reversing at the imbalance.
			ir.Reaction = Reversal
		case first == Above && second == Below && third == Above && fourth == Below:
			// If price is consistently closing aimlessly above and below an imbalance it is chopping.
			ir.Reaction = Chop
		default:
			ir.Reaction = Chop
		}
	case Resistance:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the imbalance it is chopping.
			ir.Reaction = Chop
		case above == 0:
			// If price consistently stayed below an imabalance acting as resistance it tagged then
			// it is likely reversing at the imbalance.
			ir.Reaction = Reversal
		case first == Below && third == Above && fourth == Above:
			// If price was below an imbalance acting as resistance but starts to consistently close
			// above it then it is likely breaking the imabalance.
			ir.Reaction = Break
		case first == Below && second == Below && third == Below && fourth == Above:
			// If price was below an imbalance acting as resistance but turns sharply to close above it
			// then it is likely breaking the imbalance.
			ir.Reaction = Break
		case first == Below && above > 0 && fourth == Below:
			// If price was below an imbalance acting as resistance but closed above it briefly and pushed
			// back below it then it is likely breaking the imbalance.
			ir.Reaction = Reversal
		case first == Below && second == Above && third == Below && fourth == Above:
			// If price is consistently closing aimlessly above and below an imbalance it is chopping.
			ir.Reaction = Chop
		default:
			ir.Reaction = Chop
		}
	}

	return ir, nil
}
