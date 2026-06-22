package garminsyncstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a sync run does not exist.
var ErrNotFound = errors.New("sync run not found")

// Repo persists sync runs.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders the window dates as YYYY-MM-DD text so they round-trip as
// strings (matching the project-wide date convention).
const selectCols = `id, started_at, finished_at, status, ` +
	`to_char(window_from, 'YYYY-MM-DD') AS window_from, ` +
	`to_char(window_to, 'YYYY-MM-DD') AS window_to, ` +
	`error, created_at, updated_at`

// Open inserts a new run with status='running' and the supplied rolling window
// (either may be nil), returning the created row.
func (r *Repo) Open(ctx context.Context, windowFrom, windowTo *string) (*SyncRun, error) {
	row := r.q.QueryRow(ctx, `
        INSERT INTO sync_runs (window_from, window_to)
        VALUES ($1::date, $2::date)
        RETURNING `+selectCols,
		windowFrom, windowTo,
	)
	return scanRow(row)
}

// Close sets a run's terminal status (success|error), stamps finished_at, and
// records an optional error message. Returns ErrNotFound when no row matches.
func (r *Repo) Close(ctx context.Context, id uuid.UUID, status string, errMsg *string) (*SyncRun, error) {
	row := r.q.QueryRow(ctx, `
        UPDATE sync_runs
        SET status = $2, error = $3, finished_at = now(), updated_at = now()
        WHERE id = $1
        RETURNING `+selectCols,
		id, status, errMsg,
	)
	return scanRow(row)
}

// Latest returns the most recent run by started_at, or ErrNotFound when none.
func (r *Repo) Latest(ctx context.Context) (*SyncRun, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM sync_runs ORDER BY started_at DESC LIMIT 1`)
	return scanRow(row)
}

// LastSuccessfulAt returns the finished_at of the newest successful run, or nil
// when none has ever succeeded.
func (r *Repo) LastSuccessfulAt(ctx context.Context) (*time.Time, error) {
	var ts *time.Time
	err := r.q.QueryRow(ctx,
		`SELECT finished_at FROM sync_runs WHERE status = 'success' ORDER BY finished_at DESC LIMIT 1`,
	).Scan(&ts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("last successful sync: %w", err)
	}
	return ts, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(s scanner) (*SyncRun, error) {
	var (
		run       SyncRun
		statusStr string
	)
	err := s.Scan(
		&run.ID, &run.StartedAt, &run.FinishedAt, &statusStr,
		&run.WindowFrom, &run.WindowTo, &run.Error,
		&run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan sync run: %w", err)
	}
	run.Status = Status(statusStr)
	return &run, nil
}
