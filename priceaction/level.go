package priceaction

import "github.com/dnldd/entry/shared"

// LevelKind represents the type of level.
type LevelKind int

const (
	Support LevelKind = iota
	Resistance
)

// String stringifies the provided level.
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
	Level         *Level
	PriceMovement []Movement
	Reaction      Reaction
}

// todo: generate a complete price level reaction using the constructor.
func NewPriceLevelReaction(level *Level, data []*shared.Candlestick) *PriceLevelReaction {
	plr := &PriceLevelReaction{
		Level:         level,
		PriceMovement: make([]Movement, 0, len(data)),
	}

	// Generate price movement data from the level and provided price data.
	for idx := range data {
		candle := data[idx]

		switch {
		case level.Kind == Support && candle.Close > level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Above)
		case level.Kind == Support && candle.Close <= level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Below)
		case level.Kind == Resistance && candle.Close > level.Price:
			plr.PriceMovement = append(plr.PriceMovement, Above)
		case level.Kind == Resistance && candle.Close <= level.Price:
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
	last := plr.PriceMovement[len(plr.PriceMovement)-1]

	switch {
	case above == 0 && level.Kind == Resistance:
		plr.Reaction = Reversal
	case below == 0 && level.Kind == Support:
		plr.Reaction = Reversal
	case below >= 2 && first == Above && last == Below && level.Kind == Support:
		plr.Reaction = Break
	case above >= 2 && first == Below && last == Above && level.Kind == Resistance:
		plr.Reaction = Break
		// todo: add more cases.
	default:
		plr.Reaction = Chop
	}

	return plr
}
