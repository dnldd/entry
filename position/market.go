package position

import (
	"fmt"
	"sync"

	"github.com/dnldd/entry/shared"
	"go.uber.org/atomic"
)

// Market tracks positions for the provided market.
type Market struct {
	market      string
	positions   map[string]*Position
	positionMtx sync.RWMutex
	skew        atomic.Uint32
}

// NewMarket initializes a new market.
func NewMarket(market string) *Market {
	return &Market{
		market:    market,
		positions: make(map[string]*Position),
	}
}

// AddPosition adds the provided position to the market.
func (m *Market) AddPosition(position *Position) error {
	if position == nil {
		return fmt.Errorf("position cannot be nil")
	}
	if position.Market != m.market {
		return fmt.Errorf("unexpected position market provided: %s", position.Market)
	}

	updatedSkew := shared.NeutralSkew
	currentSkew := shared.MarketSkew(m.skew.Load())
	switch currentSkew {
	case shared.NeutralSkew:
		// If the state of the market has neutral skew, the position to be tracked sets the skew
		// of the market. Once set the skew has to be unwound fully back to neutral before a
		// new skew can be set.
		switch position.Direction {
		case shared.Long:
			updatedSkew = shared.LongSkewed
		case shared.Short:
			updatedSkew = shared.ShortSkewed
		}

	case shared.LongSkewed:
		// If managing longs the market can only add more long positions, no short positions can be
		// added until all long positions have been concluded.
		switch position.Direction {
		case shared.Short:
			return fmt.Errorf("short position provided to market currently managing longs: %s", m.market)
		case shared.Long:
			// do nothing.
		}

	case shared.ShortSkewed:
		// If managing shorts the market can only add more short positions, no long positions can be
		// added until all short positions have been concluded.
		switch position.Direction {
		case shared.Long:
			return fmt.Errorf("long position provided to market currently managing shorts: %s", m.market)
		case shared.Short:
			// do nothing.
		}
	}

	// Ensure the provided position is not already tracked.
	m.positionMtx.RLock()
	_, ok := m.positions[position.ID]
	m.positionMtx.RUnlock()

	if ok {
		// do nothing if the position is already tracked.
		return nil
	}

	m.positionMtx.Lock()
	m.positions[position.ID] = position
	m.positionMtx.Unlock()

	if updatedSkew != currentSkew {
		m.skew.Store(uint32(updatedSkew))
	}

	return nil
}

// Update updates tracked positions with the market data.
func (m *Market) Update(candle *shared.Candlestick) error {
	m.positionMtx.RLock()
	defer m.positionMtx.RUnlock()

	for k := range m.positions {
		_, err := m.positions[k].UpdatePNLPercent(candle.Close)
		if err != nil {
			return fmt.Errorf("updating position PNL percents: %v", err)
		}
	}

	return nil
}

// ClosePositions closes
func (m *Market) ClosePositions(signal *shared.ExitSignal) ([]*Position, error) {
	if signal.Market != m.market {
		return nil, fmt.Errorf("unexpected %s exit signal provided for %s market", signal.Market, m.market)
	}

	m.positionMtx.Lock()
	defer m.positionMtx.Unlock()

	set := make([]*Position, 0, len(m.positions))
	for k := range m.positions {
		if m.positions[k].Direction != signal.Direction {
			// do nothing.
			continue
		}

		m.positions[k].UpdatePNLPercent(signal.Price)
		m.positions[k].ClosePosition(signal)

		set = append(set, m.positions[k])

		delete(m.positions, k)
	}

	// Reset the market status to neutral if all positions have been removed.
	if len(m.positions) == 0 {
		m.skew.Store(uint32(shared.NeutralSkew))
	}

	return set, nil
}
