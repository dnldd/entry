package position

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

const (
	// maxPositionsPurgeDuration is the maximum time closed position will be kept around for before being purged.
	maxPositionsPurgeDuration = time.Hour * 48
)

var (
	// positionsHeaderCSV is the header used for position csv files.
	positionsHeaderCSV = []string{"id", "market", "timeframe", "direction", "stoploss",
		"stoplosspointsrange", "pnlpercent", "entryprice", "entryreasons", "exitprice",
		"exitreasons", "status", "createdon", "closedon"}
)

type MarketConfig struct {
	// The tracked market.
	Market string
	// JobScheduler represents the job scheduler.
	JobScheduler *gocron.Scheduler
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// Market tracks positions for the provided market.
type Market struct {
	cfg         *MarketConfig
	positions   map[string]*Position
	positionMtx sync.RWMutex
	skew        atomic.Uint32
}

// NewMarket initializes a new market.
func NewMarket(cfg *MarketConfig) (*Market, error) {
	mkt := &Market{
		cfg:       cfg,
		positions: make(map[string]*Position),
	}

	// Schedule closed positions purge job.
	_, err := cfg.JobScheduler.Every(6).Hours().
		Do(func() {
			err := mkt.PurgeClosedPositionsJob()
			if err != nil {
				mkt.cfg.Logger.Error().Err(err).Send()
			}
		})
	if err != nil {
		return nil, fmt.Errorf("scheduling closed positions purge job for %s: %v", mkt.cfg.Market, err)
	}

	return mkt, nil
}

// AddPosition adds the provided position to the market.
func (m *Market) AddPosition(position *Position) error {
	if position == nil {
		return fmt.Errorf("position cannot be nil")
	}
	if position.Market != m.cfg.Market {
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
			return fmt.Errorf("short position provided to market currently managing longs: %s", m.cfg.Market)
		case shared.Long:
			// do nothing.
		}

	case shared.ShortSkewed:
		// If managing shorts the market can only add more short positions, no long positions can be
		// added until all short positions have been concluded.
		switch position.Direction {
		case shared.Long:
			return fmt.Errorf("long position provided to market currently managing shorts: %s", m.cfg.Market)
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
	if signal.Market != m.cfg.Market {
		return nil, fmt.Errorf("unexpected %s exit signal provided for %s market", signal.Market, m.cfg.Market)
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
	}

	// Update the market skew based on remaining open positions.
	openPositionSkew := shared.NeutralSkew
	for k := range m.positions {
		position := m.positions[k]
		if position.ClosedOn.IsZero() {
			switch position.Direction {
			case shared.Long:
				openPositionSkew = shared.LongSkewed
			case shared.Short:
				openPositionSkew = shared.ShortSkewed
			}

			break
		}
	}

	// Reset the market status to neutral if all positions have been removed.
	m.skew.Store(uint32(openPositionSkew))

	return set, nil
}

// PurgeClosedPositionsJob purges old closed positions from the provided market.
//
// This job should be run periodically.
func (m *Market) PurgeClosedPositionsJob() error {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return fmt.Errorf("fetching new york time: %v", err)
	}

	m.positionMtx.Lock()
	defer m.positionMtx.Unlock()

	for k := range m.positions {
		if m.positions[k].ClosedOn.IsZero() {
			continue
		}

		// Delete old closed positions.
		if now.Sub(m.positions[k].ClosedOn) > maxPositionsPurgeDuration {
			delete(m.positions, k)
		}
	}

	return nil
}

// PersistPositionsCSV writes the tracked positions of the provided market to file as csv.
func (m *Market) PersistPositionsCSV() (string, error) {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return "", fmt.Errorf("fetching new york time: %v", err)
	}

	filename := fmt.Sprintf("%s-positions@%s.csv", m.cfg.Market, now.Format(time.RFC3339))
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("creating positions CSV file: %v", err)
	}

	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write the CSV header to file.
	writer.Write(positionsHeaderCSV)

	// Write the position records to file.
	m.positionMtx.RLock()
	defer m.positionMtx.RUnlock()

	record := make([]string, 14)
	resetRecord := func() {
		for i := range record {
			record[i] = ""
		}
	}

	for idx := range m.positions {
		position := m.positions[idx]

		record[0] = position.ID
		record[1] = position.Market
		record[2] = position.Timeframe.String()
		record[3] = position.Direction.String()
		record[4] = strconv.FormatFloat(position.StopLoss, 'f', 3, 64)
		record[5] = strconv.FormatFloat(position.StopLossPointsRange, 'f', 3, 64)
		record[6] = strconv.FormatFloat(position.PNLPercent, 'f', 3, 64)
		record[7] = strconv.FormatFloat(position.EntryPrice, 'f', 3, 64)
		record[8] = position.EntryReasons
		record[9] = strconv.FormatFloat(position.ExitPrice, 'f', 3, 64)
		if position.ExitReasons == "" {
			record[10] = "–"
		} else {
			record[10] = position.ExitReasons
		}
		record[11] = position.Status.String()
		record[12] = position.CreatedOn.Format(time.RFC1123)
		if position.ClosedOn.IsZero() {
			record[13] = "–"
		} else {
			record[13] = position.ClosedOn.Format(time.RFC1123)
		}

		err = writer.Write(record)
		if err != nil {
			return "", fmt.Errorf("writing position record: %v", err)
		}

		resetRecord()
	}

	return filename, nil
}
