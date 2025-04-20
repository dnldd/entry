package position

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dnldd/entry/shared"
)

// MarketStatus represents defines the possible market status states.
type MarketStatus int

const (
	Neutral MarketStatus = iota
	LongInclined
	ShortInclined
)

// Market tracks positions for the provided market.
type Market struct {
	market      string
	positions   map[string]*Position
	positionMtx sync.RWMutex
	status      atomic.Uint32
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

	inclination := Neutral
	status := MarketStatus(m.status.Load())
	switch status {
	case Neutral:
		// If the state of the market is neutral, the position to be tracked sets the inclination
		// of the market. Once set the inclination has to be unwound fully back to neutral before a
		// new inclination can be set.
		switch position.Direction {
		case shared.Long:
			inclination = LongInclined
		case shared.Short:
			inclination = ShortInclined
		}

	case LongInclined:
		// If managing longs the market can only add more long positions, no short positions can be
		// added until all long positions have been concluded.
		switch position.Direction {
		case shared.Short:
			return fmt.Errorf("short position provided to market currently managing longs: %s", m.market)
		case shared.Long:
			inclination = LongInclined
		}

	case ShortInclined:
		// If managing shorts the market can only add more short positions, no long positions can be
		// added until all short positions have been concluded.
		switch position.Direction {
		case shared.Long:
			return fmt.Errorf("long position provided to market currently managing shorts: %s", m.market)
		case shared.Short:
			inclination = ShortInclined
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

	if inclination != status {
		m.status.Store(uint32(inclination))
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
		m.status.Store(uint32(Neutral))
	}

	return set, nil
}
