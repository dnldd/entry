package shared

import (
	"fmt"
	"time"

	"go.uber.org/atomic"
)

const (
	// maxBreaks is the maximum number of breaks that renders a level void.
	maxBreaks = 3
)

// LevelKind represents the type of level.
type LevelKind int

const (
	Support LevelKind = iota
	Resistance
)

// String stringifies the provided level kind.
func (l LevelKind) String() string {
	switch l {
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
	Market      string
	Price       float64
	Kind        LevelKind
	Reversals   atomic.Uint32
	Breaks      atomic.Uint32
	Breaking    atomic.Bool
	Invalidated atomic.Bool
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

// ApplyReaction applies the price reaction to the provided level.
func (l *Level) ApplyPriceReaction(reaction PriceReaction) {
	switch reaction {
	case Chop:
		// do nothing.
	case Reversal:
		l.Reversals.Add(1)
	case Break:
		if !l.Breaking.Load() {
			l.Breaking.Store(true)
		}
	}
}

// Update updates the level status based on the provided candle's close.
func (l *Level) Update(candle *Candlestick) {
	if !l.Breaking.Load() {
		return
	}

	// Confirm the break if the candle closes below a support or above a resistance.
	if (l.Kind == Support && candle.Close < l.Price) ||
		(l.Kind == Resistance && candle.Close > l.Price) {
		l.Breaks.Add(1)
		l.Reversals.Store(0)

		if l.Breaks.Load() >= maxBreaks && !l.Invalidated.Load() {
			l.Invalidated.Store(true)
		}

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
	return l.Invalidated.Load()
}

// LevelReaction describes the reaction of price at a level.
type LevelReaction struct {
	Market        string
	Level         *Level
	Timeframe     Timeframe
	PriceMovement []Movement
	CurrentPrice  float64
	Reaction      Reaction
	Status        chan StatusCode
	CreatedOn     time.Time
}

// NewLevelReaction initializes a new level reaction from the provided level and
// candlestick data.
func NewLevelReaction(market string, level *Level, data []*Candlestick) (*LevelReaction, error) {
	if len(data) != PriceDataPayloadSize {
		return nil, fmt.Errorf("price data is not the expected size: %d != expected(%d)",
			len(data), PriceDataPayloadSize)
	}

	plr := &LevelReaction{
		Market:        market,
		Level:         level,
		Timeframe:     data[len(data)-1].Timeframe,
		PriceMovement: make([]Movement, 0, len(data)),
		Status:        make(chan StatusCode, 1),
		CurrentPrice:  data[len(data)-1].Close,
		CreatedOn:     data[len(data)-1].Date,
	}

	// Generate price movement data from the level and provided price data.
	for idx := range data {
		candle := data[idx]

		switch {
		case candle.Close > level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Above)
		case candle.Close < level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Below)
		case candle.Close == level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Equal)
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

	// The level reaction is currently rooted in being able to make a decision
	// on a reaction using 4 5-minute candles. Changing the data size would
	// require reworking the logic here.

	first := plr.PriceMovement[0]
	second := plr.PriceMovement[1]
	third := plr.PriceMovement[2]
	fourth := plr.PriceMovement[3]

	switch level.Kind {
	case Support:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the level it is chopping.
			plr.Reaction = Chop
		case below == 0:
			// If price consistently stayed below a support level it tagged then it
			// it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Above && third == Below && fourth == Below:
			// If price was above a support level but starts to consistently close below it
			// then it is likely breaking the level.
			plr.Reaction = Break
		case first == Above && second == Above && third == Above && fourth == Below:
			// If price was above a support but turns sharply to close below it then
			// it is likely breaking the level.
			plr.Reaction = Break
		case first == Above && below > 0 && fourth == Above:
			// If price was above a support level but closed below it briefly and pushed back
			// above it then it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Above && second == Below && third == Above && fourth == Below:
			// If price is consistently closing aimlessly above and below a level it is chopping.
			plr.Reaction = Chop
		default:
			plr.Reaction = Chop
		}
	case Resistance:
		switch {
		case above == 0 && below == 0:
			// If price is not closing above or below the level it is chopping.
			plr.Reaction = Chop
		case above == 0:
			// If price consistently stayed below a resistance level it tagged then
			// it is likely reversing at the level.
			plr.Reaction = Reversal
		case first == Below && third == Above && fourth == Above:
			// If price was below a resistance level but starts to consistently close above it
			// then it is likely breaking the level.
			plr.Reaction = Break
		case first == Below && second == Below && third == Below && fourth == Above:
			// If price was below a resistance but turns sharply to close above it then it is
			// likely breaking the level.
			plr.Reaction = Break
		case first == Below && above > 0 && fourth == Below:
			// If price was below a resistance level but closed above it briefly and pushed
			// back below it then it is likely breaking the level.
			plr.Reaction = Reversal
		case first == Below && second == Above && third == Below && fourth == Above:
			// If price is consistently closing aimlessly above and below a level it is chopping.
			plr.Reaction = Chop
		default:
			plr.Reaction = Chop
		}
	}

	return plr, nil
}

// ApplyReaction applies the level reaction to the associated level.
func (l *LevelReaction) ApplyReaction() {
	l.Level.ApplyReaction(l.Reaction)
}
