package coachrecs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a recommendation does not exist.
var ErrNotFound = errors.New("coach recommendation not found")

// Repo persists coach recommendations.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders date as YYYY-MM-DD text so it round-trips as a string
// (matching the training_plans.start_date convention).
const selectCols = `id, to_char(date, 'YYYY-MM-DD') AS date, scope, recommendation, reason, created_at, updated_at`

// Insert creates a coach_recommendations row, populating generated fields on rec.
func (r *Repo) Insert(ctx context.Context, rec *Recommendation) error {
	row := r.q.QueryRow(ctx, `
        INSERT INTO coach_recommendations (date, scope, recommendation, reason)
        VALUES ($1::date, $2, $3, $4)
        RETURNING `+selectCols,
		rec.Date, rec.Scope, rec.Recommendation, rec.Reason,
	)
	return scanInto(row, rec)
}

// GetByID returns a single recommendation.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Recommendation, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM coach_recommendations WHERE id = $1`, id)
	return scanRow(row)
}

// Delete removes a recommendation. Returns ErrNotFound when no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM coach_recommendations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete coach recommendation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListWindow returns recommendations whose date falls in the inclusive local-date
// window [from, to], optionally narrowed to one scope, ordered newest-first.
func (r *Repo) ListWindow(ctx context.Context, from, to string, scope *string) ([]*Recommendation, error) {
	q := `SELECT ` + selectCols + ` FROM coach_recommendations WHERE date >= $1::date AND date <= $2::date`
	args := []any{from, to}
	if scope != nil {
		q += ` AND scope = $3`
		args = append(args, *scope)
	}
	q += ` ORDER BY date DESC, created_at DESC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list coach recommendations: %w", err)
	}
	defer rows.Close()
	var out []*Recommendation
	for rows.Next() {
		rec, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(s scanner) (*Recommendation, error) {
	var rec Recommendation
	if err := scanInto(s, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func scanInto(s scanner, rec *Recommendation) error {
	err := s.Scan(&rec.ID, &rec.Date, &rec.Scope, &rec.Recommendation, &rec.Reason, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("scan coach recommendation: %w", err)
	}
	return nil
}
