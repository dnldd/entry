package fetch

import (
	"fmt"
	"os"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
)

// HistoricDataConfig represents the historic data source configuration.
type HistoricDataConfig struct {
	// Market represents the historic data market.
	Market string
	// Timeframe represents the timeframe for the historic data.
	Timeframe shared.Timeframe
	// FilePath is the filepath to the historic market data.
	FilePath string
	// SignalCaughtUp signals a market is caught up on market data.
	SignalCaughtUp func(signal shared.CaughtUpSignal)
	// SendMarketUpdate relays the provided market update for processing.
	SendMarketUpdate func(candle shared.Candlestick)
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// HistoricData represents historic market data.
type HistoricData struct {
	cfg     *HistoricDataConfig
	candles []shared.Candlestick
}

// loadHistoricData loads the historic data bytes from the provided file path.
func loadHistoricData(filepath string) ([]gjson.Result, error) {
	readb, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("reading historic data from file with path '%s': %v", filepath, err)
	}

	b := gjson.GetBytes(readb, "").Array()

	return b, nil
}

// NewhistoricData initializes a new historic data source.
func NewHistoricData(cfg *HistoricDataConfig) (*HistoricData, error) {
	b, err := loadHistoricData(cfg.FilePath)
	if err != nil {
		return nil, fmt.Errorf("loading historic data: %v", err)
	}

	historicData := HistoricData{
		cfg: cfg,
	}

	candles, err := shared.ParseCandlesticks(b, cfg.Market, cfg.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("parsing candlesticks: %v", err)
	}

	historicData.candles = candles

	return &historicData, nil
}

// ProcessHistoricalData streams historical data for a market.
func (h *HistoricData) ProcessHistoricalData() error {
	// Determine the range for the data provided.
	first := h.candles[0].Date
	last := h.candles[len(h.candles)-1].Date
	timeDiffInHours := last.Sub(first).Hours()

	h.cfg.Logger.Info().Msgf("processing historical data covering %.2f hours, from %s, to %s",
		timeDiffInHours, first.Format(time.RFC1123), last.Format(time.RFC1123))

	// Find the current session and use its close to determine when to signal the market has caught up.
	_, currentSession, err := shared.CurrentSession(first)
	if err != nil {
		return fmt.Errorf("fetching current session: %v", err)
	}

	var caughtUp bool
	for idx := range h.candles {
		candle := h.candles[idx]
		if candle.Date.After(currentSession.Close) && !caughtUp {
			// Send a caught up signal immediately the current session closes.
			h.cfg.SignalCaughtUp(shared.CaughtUpSignal{
				Market: h.cfg.Market,
				Status: make(chan shared.StatusCode, 1),
			})

			caughtUp = true
		}

		// Process historical data synchroniously.
		h.cfg.SendMarketUpdate(candle)
		select {
		case <-candle.Status:
		case <-time.After(time.Second * 3):
			return fmt.Errorf("timed out processing market update: %v", err)
		}
	}

	return nil
}
