package wellness

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no entry exists for a date.
var ErrNotFound = errors.New("wellness entry not found")

const dateFormat = "2006-01-02"

// Repo persists wellness entries against a pgxpool.Pool or a pgx.Tx.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `entry_date, fatigue, soreness, stress, mood, motivation, note, created_at, updated_at`

// Upsert writes the entry for `date`, replacing every field with what's on e
// (nil pointers become NULL — PUT full-replace semantics). created_at is
// preserved across replacements; updated_at advances.
func (r *Repo) Upsert(ctx context.Context, date time.Time, e *Entry) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO wellness_entries
		   (entry_date, fatigue, soreness, stress, mood, motivation, note, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		 ON CONFLICT (entry_date) DO UPDATE SET
		   fatigue    = EXCLUDED.fatigue,
		   soreness   = EXCLUDED.soreness,
		   stress     = EXCLUDED.stress,
		   mood       = EXCLUDED.mood,
		   motivation = EXCLUDED.motivation,
		   note       = EXCLUDED.note,
		   updated_at = now()`,
		date, e.Fatigue, e.Soreness, e.Stress, e.Mood, e.Motivation, e.Note,
	)
	if err != nil {
		return fmt.Errorf("upsert wellness entry: %w", err)
	}
	return nil
}

// Get returns the entry for `date`, or ErrNotFound when no row exists.
func (r *Repo) Get(ctx context.Context, date time.Time) (*Entry, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM wellness_entries WHERE entry_date = $1`,
		date,
	)
	e, err := scanEntry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan wellness entry: %w", err)
	}
	return e, nil
}

// List returns entries whose date falls within [from, to] inclusive, ascending.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM wellness_entries
		 WHERE entry_date BETWEEN $1 AND $2
		 ORDER BY entry_date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("query wellness entries: %w", err)
	}
	defer rows.Close()

	out := make([]*Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan wellness entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wellness entries: %w", err)
	}
	return out, nil
}

// Delete removes the entry for `date`, returning ErrNotFound when none existed.
func (r *Repo) Delete(ctx context.Context, date time.Time) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM wellness_entries WHERE entry_date = $1`, date)
	if err != nil {
		return fmt.Errorf("delete wellness entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanRow is satisfied by both pgx.Row and pgx.Rows.
type scanRow interface {
	Scan(dest ...any) error
}

func scanEntry(row scanRow) (*Entry, error) {
	var (
		e    Entry
		date time.Time
	)
	if err := row.Scan(
		&date, &e.Fatigue, &e.Soreness, &e.Stress, &e.Mood, &e.Motivation, &e.Note,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, err
	}
	e.Date = date.Format(dateFormat)
	return &e, nil
}
