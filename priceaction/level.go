package priceaction

import "github.com/dnldd/entry/shared"

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
func NewLevel(market string, price float64, candle *shared.Candlestick) *Level {
	lvl := &Level{
		Market: market,
		Price:  price,
	}

	switch {
	case price >= candle.High:
		lvl.Kind = Resistance
	case price <= candle.Low:
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

// PriceLevelReaction describes the reaction of price at a level.
type PriceLevelReaction struct {
	Market        string
	Level         *Level
	PriceMovement []Movement
	Reaction      Reaction
}

// NewPriceLevelReaction initializes a new price level reaction from the provided level and
// candlestick data.
func NewPriceLevelReaction(market string, level *Level, data []*shared.Candlestick) *PriceLevelReaction {
	plr := &PriceLevelReaction{
		Market:        market,
		Level:         level,
		PriceMovement: make([]Movement, 0, len(data)),
	}

	// Generate price movement data from the level and provided price data.
	for idx := range data {
		candle := data[idx]

		switch {
		case level.Kind == Support && candle.Close > level.Price:
			plr.PriceMovement[idx] = Above
		case level.Kind == Support && candle.Close <= level.Price:
			plr.PriceMovement[idx] = Below
		case level.Kind == Resistance && candle.Close > level.Price:
			plr.PriceMovement[idx] = Above
		case level.Kind == Resistance && candle.Close <= level.Price:
			plr.PriceMovement[idx] = Below
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

	switch {
	case above == 0 && level.Kind == Resistance:
		// If price consistently stayed below a resistance level it tagged then
		// it is likely setting up for a reversal.
		plr.Reaction = Reversal
	case below == 0 && level.Kind == Support:
		// If price consistently stayed above a a support level it tagged then it
		// it is likely setting up for a reversal.
		plr.Reaction = Reversal
	case first == Above && lastButOne == Below && last == Below && level.Kind == Support:
		// If price was above a support level but starts to consistently close below it
		// then it is likely breaking the level.
		plr.Reaction = Break
	case first == Below && lastButOne == Above && last == Above && level.Kind == Resistance:
		// If price was below a resistance level but starts to consistently close above it
		// then it is likely breaking the level.
		plr.Reaction = Break
	case first == Above && lastButOne == Above && last == Below && level.Kind == Support:
		// If price was above a support but turns sharply to close below it then
		// it is likely breaking the level.
		plr.Reaction = Break
	case first == Below && lastButOne == Below && last == Above && level.Kind == Resistance:
		// If price was below a resistance but turns sharply to close above it then it is
		// likely breaking the level.
		plr.Reaction = Break
	case first == Above && below > 0 && last == Above && level.Kind == Support:
		// If price was above a support level but closed below it briefly and pushed back
		// above it then it is likely setting up a reversal.
		plr.Reaction = Reversal
	case first == Below && above > 0 && last == Below && level.Kind == Resistance:
		// If price was below a resistance level but closed above it briefly and pushed
		// back below it then it is likely setting up a reversal.
	default:
		plr.Reaction = Chop
	}

	return plr
}

// PriceLevelReactionsSignal relays price level reactions for processing.
type PriceLevelReactionsSignal struct {
	Market    string
	Reactions []*PriceLevelReaction
}
