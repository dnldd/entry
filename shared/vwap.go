package shared

import (
	"fmt"
	"time"
)

// VWAP represents a unit VWAP entry for a market.
type VWAP struct {
	Value float64
	Date  time.Time
}

// ReactionAtVWAP describes the reaction of price relative to vwap.
type ReactionAtVWAP struct {
	ReactionAtFocus
	VWAPData []*VWAP
}

// fetchVWAPLevelKind returns the level kind status of the provided vwap.
func fetchVWAPLevelKind(vwap *VWAP, candle *Candlestick) LevelKind {
	var levelKind LevelKind
	if vwap.Value > candle.Close {
		levelKind = Resistance
	} else {
		levelKind = Support
	}

	return levelKind
}

// NewReactionAtVWAP initializes a new reaction from the provided vwap and candlestick data.
func NewReactionAtVWAP(market string, vwapData []*VWAP, priceData []*Candlestick) (*ReactionAtVWAP, error) {
	if len(vwapData) != VWAPDataPayloadSize {
		return nil, fmt.Errorf("vwap data is not the expected size: %d != expected(%d)",
			len(vwapData), VWAPDataPayloadSize)
	}

	if len(priceData) != PriceDataPayloadSize {
		return nil, fmt.Errorf("price data is not the expected size: %d != expected(%d)",
			len(priceData), PriceDataPayloadSize)
	}

	if len(vwapData) != len(priceData) {
		return nil, fmt.Errorf("data length mismatch, %d != %d", len(vwapData), len(priceData))
	}

	levelKind := fetchVWAPLevelKind(vwapData[0], priceData[0])
	vr := &ReactionAtVWAP{
		ReactionAtFocus: ReactionAtFocus{
			Market:        market,
			LevelKind:     levelKind,
			Timeframe:     priceData[len(priceData)-1].Timeframe,
			PriceMovement: make([]PriceMovement, 0, len(priceData)),
			Status:        make(chan StatusCode, 1),
			CurrentPrice:  priceData[len(priceData)-1].Close,
			CreatedOn:     priceData[len(priceData)-1].Date,
		},
		VWAPData: vwapData,
	}

	// Generate price movement data from the level and provided price data.
	for idx := range priceData {
		candle := priceData[idx]
		vwap := vwapData[idx]

		switch {
		case candle.Close > vwap.Value:
			vr.PriceMovement = append(vr.PriceMovement, Above)
		case candle.Close < vwap.Value:
			vr.PriceMovement = append(vr.PriceMovement, Below)
		case candle.Close == vwap.Value:
			vr.PriceMovement = append(vr.PriceMovement, Equal)
		}
	}

	// Generate a price reaction based on the price movement data.
	var above, below uint32
	for idx := range vr.PriceMovement {
		switch {
		case vr.PriceMovement[idx] == Above:
			above++
		case vr.PriceMovement[idx] == Below:
			below++
		}
	}

	// The vwap reaction is currently rooted in being able to make a decision
	// on a reaction using 4 5-minute candles. Changing the data size would
	// require reworking the logic here.

	first := vr.PriceMovement[0]
	second := vr.PriceMovement[1]
	third := vr.PriceMovement[2]
	fourth := vr.PriceMovement[3]

	switch levelKind {
	case Support:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the vwap it is chopping.
			vr.Reaction = Chop
		case below == 0:
			// If price consistently stayed below a support vwap it tagged then it
			// it is likely reversing at the vwap.
			vr.Reaction = Reversal
		case first == Above && third == Below && fourth == Below:
			// If price was above a vwap acting as support but starts to consistently close below it
			// then it is likely breaking the vwap.
			vr.Reaction = Break
		case first == Above && second == Above && third == Above && fourth == Below:
			// If price was above a vwap acting as support but turns sharply to close below it then
			// it is likely breaking the vwap.
			vr.Reaction = Break
		case first == Above && below > 0 && fourth == Above:
			// If price was above a vwap acting as support level but closed below it briefly and
			// pushed back above it then it is likely reversing at the level.
			vr.Reaction = Reversal
		case first == Above && second == Below && third == Above && fourth == Below:
			// If price is consistently closing aimlessly above and below a vwap it is chopping.
			vr.Reaction = Chop
		default:
			vr.Reaction = Chop
		}
	case Resistance:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the vwap it is chopping.
			vr.Reaction = Chop
		case above == 0:
			// If price consistently stayed below a vwap acting as resistance it tagged then
			// it is likely reversing at the vwap.
			vr.Reaction = Reversal
		case first == Below && third == Above && fourth == Above:
			// If price was below a vwap acting as resistance but starts to consistently close
			// above it then it is likely breaking the vwap.
			vr.Reaction = Break
		case first == Below && second == Below && third == Below && fourth == Above:
			// If price was below a vwap acting as resistance but turns sharply to close above it
			// then it is likely breaking the vwap.
			vr.Reaction = Break
		case first == Below && above > 0 && fourth == Below:
			// If price was below a vwap acting as resistance but closed above it briefly and pushed
			// back below it then it is likely breaking the vwap.
			vr.Reaction = Reversal
		case first == Below && second == Above && third == Below && fourth == Above:
			// If price is consistently closing aimlessly above and below a level it is chopping.
			vr.Reaction = Chop
		default:
			vr.Reaction = Chop
		}
	}

	return vr, nil
}
