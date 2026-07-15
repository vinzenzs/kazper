package supplements

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no entry exists for an id.
var ErrNotFound = errors.New("supplement entry not found")

// Repo persists supplement entries against a pgxpool.Pool or a pgx.Tx.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo { return &Repo{q: q} }

const selectCols = `id, logged_at, name, dose, dose_unit, note, created_at, updated_at`

// Insert creates an entry and returns it.
func (r *Repo) Insert(ctx context.Context, e *Entry) (*Entry, error) {
	id := uuid.New()
	_, err := r.q.Exec(ctx,
		`INSERT INTO supplement_entries (id, logged_at, name, dose, dose_unit, note)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, e.LoggedAt, e.Name, e.Dose, e.DoseUnit, e.Note,
	)
	if err != nil {
		return nil, fmt.Errorf("insert supplement entry: %w", err)
	}
	return r.GetByID(ctx, id)
}

// GetByID returns the entry, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM supplement_entries WHERE id = $1`, id)
	e, err := scanEntry(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan supplement entry: %w", err)
	}
	return e, nil
}

// List returns entries whose logged_at falls in [from, to), ascending.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM supplement_entries
		 WHERE logged_at >= $1 AND logged_at < $2
		 ORDER BY logged_at ASC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query supplement entries: %w", err)
	}
	defer rows.Close()
	out := make([]*Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan supplement entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Delete removes the entry, returning ErrNotFound when absent.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM supplement_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete supplement entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanRow interface{ Scan(dest ...any) error }

func scanEntry(row scanRow) (*Entry, error) {
	var e Entry
	if err := row.Scan(&e.ID, &e.LoggedAt, &e.Name, &e.Dose, &e.DoseUnit, &e.Note, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}
