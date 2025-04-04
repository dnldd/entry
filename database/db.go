package database

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dnldd/entry/position"
	"github.com/dnldd/entry/shared"
	rqlitehttp "github.com/rqlite/rqlite-go-http"
	"github.com/rs/zerolog"
)

const (
	// SQL statements.
	createPositionTableSQL   = "CREATE TABLE position IF NOT EXISTS (id TEXT PRIMARY KEY, market TEXT, timeframe TEXT, direction INTEGER, stoploss INTERGER, pnlpercent INTEGER, entryprice INTEGER, entryreasons TEXT, exitprice INTEGER, exitreasons TEXT, status INTEGER, createdon INTEGER, closedon INTEGER)"
	createMetadataSQL        = "CREATE TABLE metadata IF NOT EXISTS (id TEXT PRIMARY KEY, total INTEGER, wins INTEGER, winpercent INTEGER, losses INTEGER, losspercent INTEGER, createdon INTEGER)"
	persistClosedPositionSQL = "INSERT INTO position(id, market, timeframce, direction, stoploss, pnlpercent, entryprice, entryreasons, exitprice, exitreasons, status, createdon, closedon) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)"
	findMetadataSQL          = "SELECT * FROM metadata where id = ?"
	updateMetadataSQL        = "UPDATE metadata SET total = total + 1, SET wins = wins + ?, SET winpercent = winpercent + ?, SET losses = losses + ?, losspercent = losspercent + ? WHERE id = ?"
	persistMetadataSQL       = "INSERT INTO metadata(id, total, wins, winpercent, losses, losspercent, createdon) VALUES(?,?,?,?,?,?,?)"
)

// PositionStorer defines the requirements for storing positions.
type PositionStorer interface {
	// PersistClosedPosition stores the provided closed position to the database.
	PersistClosedPosition(ctx context.Context, position *position.Position) error
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	// Endpoint represents the database connection endpoint.
	Endpoint string
	// User is the database user.
	User string
	// Pass is the database user pass.
	Pass string
	// Logger is the database logger.
	Logger *zerolog.Logger
}

// Database represents the database connection.
type Database struct {
	cfg    *DatabaseConfig
	client *rqlitehttp.Client
}

// Ensure the database implements the PositionStorer interface.
var _ PositionStorer = (*Database)(nil)

// NewDatabase initializes a new database connection.
func NewDatabase(ctx context.Context, cfg *DatabaseConfig) (*Database, error) {
	httpc := &http.Client{Timeout: time.Second * 5}
	client, err := rqlitehttp.NewClient(cfg.Endpoint, httpc)
	if err != nil {
		return nil, fmt.Errorf("Creating database client: %w", err)
	}

	client.SetBasicAuth(cfg.User, cfg.Pass)

	db := &Database{
		cfg:    cfg,
		client: client,
	}

	err = db.bootstrap(ctx)
	if err != nil {
		return nil, fmt.Errorf("Bootstrapping database: %w", err)
	}

	return db, nil
}

// bootstrap initializes the database.
func (db *Database) bootstrap(ctx context.Context) error {
	_, err := db.client.Execute(ctx, rqlitehttp.SQLStatements{
		{SQL: createMetadataSQL},
		{SQL: createPositionTableSQL},
	}, &rqlitehttp.ExecuteOptions{
		Transaction: true,
		Timings:     true,
	})
	if err != nil {
		return err
	}

	return nil
}

// generateMetadataID generates deterministic ids for metadata using the
// current month, week and market.
func generateMetadataID(currentTime time.Time, market string) string {
	month := currentTime.Month().String()
	week := currentTime.Day() / 7

	id := fmt.Sprintf("%s-Week-%d-%s", month, week, market)
	return id
}

// PersistClosedPosition stores the provided closed position to the database.
func (db *Database) PersistClosedPosition(ctx context.Context, pos *position.Position) error {
	_, err := db.client.Execute(ctx, rqlitehttp.SQLStatements{
		{
			SQL: persistClosedPositionSQL,
			PositionalParams: []any{pos.ID, pos.Market, pos.Timeframe, pos.Direction, pos.StopLoss,
				pos.PNLPercent, pos.EntryPrice, pos.EntryReasons, pos.ExitPrice, pos.ExitReasons,
				pos.Status, pos.CreatedOn, pos.ClosedOn},
		},
	}, &rqlitehttp.ExecuteOptions{Transaction: true, Timings: true})
	if err != nil {
		return err
	}

	var win, loss int
	var winpercent, losspercent float64

	switch {
	case pos.Status == position.StoppedOut && pos.PNLPercent < 0:
		loss++
		losspercent = pos.PNLPercent
	case pos.Status == position.Closed && pos.PNLPercent > 0:
		win++
		winpercent = pos.PNLPercent
	default:
		db.cfg.Logger.Error().Msgf("unexpected closed position state for metadata calculations: %s", spew.Sdump(pos))
	}

	now, _, err := shared.NewYorkTime()
	if err != nil {
		return err
	}

	id := generateMetadataID(now, pos.Market)
	resp, err := db.client.QuerySingle(ctx, findMetadataSQL, id)
	if err != nil {
		return err
	}

	exists := len(resp.GetQueryResultsAssoc()) > 0
	switch {
	case exists:
		resp, err := db.client.Execute(ctx, rqlitehttp.SQLStatements{
			{
				SQL:              updateMetadataSQL,
				PositionalParams: []any{win, winpercent, loss, losspercent, id},
			},
		}, &rqlitehttp.ExecuteOptions{Transaction: true, Timings: true})
		if err != nil {
			return err
		}
		has, idx, errStr := resp.HasError()
		if has {
			return fmt.Errorf("updating metadata %s: %d -> %s", id, idx, errStr)
		}
	default:
		now, _, err := shared.NewYorkTime()
		if err != nil {
			return err
		}

		resp, err := db.client.Execute(ctx, rqlitehttp.SQLStatements{
			{
				SQL:              persistMetadataSQL,
				PositionalParams: []any{id, 1, win, winpercent, loss, losspercent, now.Unix()},
			},
		}, &rqlitehttp.ExecuteOptions{Transaction: true, Timings: true})
		if err != nil {
			return err
		}
		has, idx, errStr := resp.HasError()
		if has {
			return fmt.Errorf("updating metadata %s: %d -> %s", id, idx, errStr)
		}
	}

	return nil
}
