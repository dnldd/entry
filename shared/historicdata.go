package shared

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
)

// HistoricDataConfig represents the historic data source configuration.
type HistoricDataConfig struct {
	// FilePath is the filepath to the historic market data.
	FilePath string
	// SignalCaughtUp signals a market is caught up on market data.
	SignalCaughtUp func(signal CaughtUpSignal)
	// SendMarketUpdate relays the provided market update to all subscribers.
	NotifySubscribers func(candle Candlestick) error
	// Logger represents the application logger.
	Logger *zerolog.Logger
}

// HistoricData represents historic market data.
type HistoricData struct {
	cfg        *HistoricDataConfig
	market     string
	location   *time.Location
	candles    []Candlestick
	candlesMtx sync.RWMutex
	timeframes []string
	startTime  time.Time
	endTime    time.Time
}

// loadHistoricData loads the historic data bytes from the provided file path.
func loadHistoricData(filepath string) (*gjson.Result, error) {
	readb, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("reading historic data from file with path '%s': %v", filepath, err)
	}

	b := gjson.ParseBytes(readb)

	return &b, nil
}

// NewhistoricData initializes a new historic data source.
func NewHistoricData(cfg *HistoricDataConfig) (*HistoricData, error) {
	b, err := loadHistoricData(cfg.FilePath)
	if err != nil {
		return nil, fmt.Errorf("loading historic data: %v", err)
	}

	market := b.Get("market").String()

	loc, err := time.LoadLocation(NewYorkLocation)
	if err != nil {
		return nil, fmt.Errorf("loading new york location: %v", err)
	}

	historicData := HistoricData{
		market:   market,
		cfg:      cfg,
		location: loc,
	}

	timeframes := []Timeframe{OneMinute, FiveMinute, OneHour}
	for idx := range timeframes {
		timeframe := timeframes[idx]

		data := b.Get(timeframe.String()).Array()
		if len(data) == 0 {
			continue
		}

		candles, err := ParseCandlesticks(data, market, timeframe, loc)
		if err != nil {
			return nil, fmt.Errorf("parsing candlesticks: %v", err)
		}

		historicData.timeframes = append(historicData.timeframes, timeframe.String())
		historicData.candles = append(historicData.candles, candles...)
	}

	// Sort the multi timeframe dats by the timestamp and timeframe.
	slices.SortFunc(historicData.candles, func(a, b Candlestick) int {
		switch {
		case a.Date.Before(b.Date):
			return -1
		case a.Date.After(b.Date):
			return 1
		case a.Date.Equal(b.Date):
			switch {
			case (a.Timeframe == OneMinute && b.Timeframe == FiveMinute) ||
				(a.Timeframe == OneMinute && b.Timeframe == OneHour) ||
				(a.Timeframe == FiveMinute && b.Timeframe == OneHour):
				return -1
			case (a.Timeframe == OneHour && b.Timeframe == OneMinute) ||
				(a.Timeframe == OneHour && b.Timeframe == FiveMinute) ||
				(a.Timeframe == FiveMinute && b.Timeframe == OneMinute):
				return 1
			default:
				return 0
			}
		}

		return 0
	})

	historicData.startTime = historicData.candles[0].Date
	historicData.endTime = historicData.candles[len(historicData.candles)-1].Date

	return &historicData, nil
}

// ProcessHistoricalData streams historical data for a market.
func (h *HistoricData) ProcessHistoricalData() error {
	h.candlesMtx.RLock()
	defer h.candlesMtx.RUnlock()

	first := h.candles[0].Date
	last := h.candles[len(h.candles)-1].Date
	timeDiffInHours := last.Sub(first).Hours()

	tfs := strings.Join(h.timeframes, ",")
	h.cfg.Logger.Info().Msgf("processing historical [%s] data covering %.2f hours, from %s, to %s",
		tfs, timeDiffInHours, first.Format(time.RFC1123), last.Format(time.RFC1123))

	// Find the current session and use its close to determine when to signal the market has caught up.
	_, currentSession, err := CurrentSession(first)
	if err != nil {
		return fmt.Errorf("fetching current session: %v", err)
	}

	var caughtUp bool
	for idx := range h.candles {
		candle := h.candles[idx]
		if candle.Date.After(currentSession.Close) && !caughtUp {
			// Send a caught up signal immediately the current session closes.
			sig := NewCaughtUpSignal(h.market)
			h.cfg.SignalCaughtUp(sig)
			<-sig.Status
			caughtUp = true
			h.cfg.Logger.Info().Msgf("caught up signal sent for %s historic data", h.market)
		}

		// Process historical data synchroniously.
		err := h.cfg.NotifySubscribers(candle)
		if err != nil {
			return fmt.Errorf("processing historical data: %v", err)
		}
	}

	return nil
}

// FetchStartTime returns the start time of the loaded historical data.
func (h *HistoricData) FetchStartTime() time.Time {
	return h.startTime
}

// FetchEndTime returns the end time of the loaded historical data.
func (h *HistoricData) FetchEndTime() time.Time {
	return h.endTime
}

// FetchMarket returns the backtest market.
func (h *HistoricData) FetchMarket() string {
	return h.market
}
