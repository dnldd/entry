package shared

import (
	"fmt"
)

const (
	// minPriceDataSize is the minimum size for price data.
	minPriceDataSize = 5
	// maxBreaks is the maximum number of breaks that renders a level void.
	maxBreaks = 2
)

// LevelKind represents the type of level.
type LevelKind int

const (
	Support LevelKind = iota
	Resistance
)

// String stringifies the provided level kind.
func (l *LevelKind) String() string {
	switch *l {
	case Support:
		return "support"
	case Resistance:
		return "resistance"
	default:
		return "unknown"
	}
}

// Level represents a support or resistance level.
type Level struct {
	Market    string
	Price     float64
	Kind      LevelKind
	Reversals uint32
	Breaks    uint32
}

// NewLevel initializes a new level.
func NewLevel(market string, price float64, candle *Candlestick) *Level {
	lvl := &Level{
		Market: market,
		Price:  price,
	}

	switch {
	case candle.Close < price:
		lvl.Kind = Resistance
	case candle.Close >= price:
		lvl.Kind = Support
	}

	return lvl
}

// Update updates the provided level based on the provided price reaction.
func (l *Level) Update(reaction Reaction) {
	switch reaction {
	case Chop:
		// do nothing.
	case Reversal:
		l.Reversals++
	case Break:
		l.Breaks++
		l.Reversals = 0

		switch l.Kind {
		case Support:
			l.Kind = Resistance
		default:
			l.Kind = Support
		}
	}
}

// IsInvalidated checks whether the provided level has been invalidated.
func (l *Level) IsInvalidated() bool {
	return l.Breaks >= maxBreaks
}

// LevelReaction describes the reaction of price at a level.
type LevelReaction struct {
	Market        string
	Level         *Level
	PriceMovement []Movement
	Reaction      Reaction
}

// NewLevelReaction initializes a new level reaction from the provided level and
// candlestick data.
func NewLevelReaction(market string, level *Level, data []*Candlestick) (*LevelReaction, error) {
	if len(data) < minPriceDataSize {
		return nil, fmt.Errorf("price data is less than expected minumum: %d < %d", len(data), minPriceDataSize)
	}

	plr := &LevelReaction{
		Market:        market,
		Level:         level,
		PriceMovement: make([]Movement, 0, len(data)),
	}

	// Generate price movement data from the level and provided price data.
	for idx := range data {
		candle := data[idx]

		switch {
		case candle.Close > level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Above)
		case candle.Close <= level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Below)
		}
	}

	// Generate a price reaction based on the price movement data.
	var above, below uint32
	for idx := range plr.PriceMovement {
		switch {
		case plr.PriceMovement[idx] == Above:
			above++
		case plr.PriceMovement[idx] == Below:
			below++
		}
	}

	first := plr.PriceMovement[0]
	lastButOne := plr.PriceMovement[len(plr.PriceMovement)-2]
	last := plr.PriceMovement[len(plr.PriceMovement)-1]

	switch level.Kind {
	case Support:
		switch {
		case below == 0:
			// If price consistently stayed above or below a support level it tagged then it
			// it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Below && lastButOne == Above && last == Above:
			// If price was below a support level but starts to consistently close above it
			// then it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Above && lastButOne == Below && last == Below:
			// If price was above a support level but starts to consistently close below it
			// then it is likely breaking the level.
			plr.Reaction = Break
		case first == Above && lastButOne == Above && last == Below:
			// If price was above a support but turns sharply to close below it then
			// it is likely breaking the level.
			plr.Reaction = Break
		case first == Above && below > 0 && last == Above:
			// If price was above a support level but closed below it briefly and pushed back
			// above it then it is likely reversing at the level.
			plr.Reaction = Reversal
		default:
			plr.Reaction = Chop
		}
	case Resistance:
		switch {
		case above == 0:
			// If price consistently stayed below a resistance level it tagged then
			// it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Above && lastButOne == Below && last == Below:
			// If price was above a resistance level but starts to consistently close below it
			// then it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Below && lastButOne == Above && last == Above:
			// If price was below a resistance level but starts to consistently close above it
			// then it is likely breaking the level.
			plr.Reaction = Break
		case first == Below && lastButOne == Below && last == Above:
			// If price was below a resistance but turns sharply to close above it then it is
			// likely breaking the level.
			plr.Reaction = Break
		case first == Below && above > 0 && last == Below:
			// If price was below a resistance level but closed above it briefly and pushed
			// back below it then it is likely breaking the level.
			plr.Reaction = Reversal
		default:
			plr.Reaction = Chop
		}
	}

	return plr, nil
}
