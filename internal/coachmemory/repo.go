package coachmemory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a memory item does not exist.
var ErrNotFound = errors.New("coach memory not found")

// Repo persists coach memory.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders the three date columns as YYYY-MM-DD text so they
// round-trip as strings (matching the training_plans.start_date convention).
const selectCols = `id, kind, text, reason, scope, ` +
	`to_char(date, 'YYYY-MM-DD') AS date, ` +
	`to_char(expires_at, 'YYYY-MM-DD') AS expires_at, ` +
	`to_char(review_at, 'YYYY-MM-DD') AS review_at, ` +
	`status, created_at, updated_at`

// Insert creates a coach_memory row, populating generated fields on m. status
// defaults to 'active' at the DB.
func (r *Repo) Insert(ctx context.Context, m *Memory) error {
	row := r.q.QueryRow(ctx, `
        INSERT INTO coach_memory (kind, text, reason, scope, date, expires_at, review_at)
        VALUES ($1, $2, $3, $4, $5::date, $6::date, $7::date)
        RETURNING `+selectCols,
		m.Kind, m.Text, m.Reason, scopeArg(m.Scope), m.Date, m.ExpiresAt, m.ReviewAt,
	)
	return scanInto(row, m)
}

// GetByID returns a single memory item.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Memory, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM coach_memory WHERE id = $1`, id)
	return scanRow(row)
}

// Delete removes a memory item. Returns ErrNotFound when no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM coach_memory WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete coach memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListParams narrows a windowed list. Recommendations are filtered to
// [From, To]; dateless standing items ignore the window. Kind/Scope narrow
// further; IncludeArchived and the expiry filter govern visibility.
type ListParams struct {
	From            string
	To              string
	Kind            *string
	Scope           *string
	IncludeArchived bool
	// AsOf is the local date used to exclude expired items (expires_at < AsOf).
	AsOf string
}

// ListWindow returns memory items newest-first. Standing items (non-recommendation
// kinds) are returned regardless of the window; recommendations are filtered to
// [From, To]. Archived and expired items are excluded unless IncludeArchived.
func (r *Repo) ListWindow(ctx context.Context, p ListParams) ([]*Memory, error) {
	q := `SELECT ` + selectCols + ` FROM coach_memory WHERE (
        kind <> 'recommendation' OR (date >= $1::date AND date <= $2::date)
    )`
	args := []any{p.From, p.To}
	next := 3
	if !p.IncludeArchived {
		q += ` AND status = 'active'`
		q += fmt.Sprintf(` AND (expires_at IS NULL OR expires_at >= $%d::date)`, next)
		args = append(args, p.AsOf)
		next++
	}
	if p.Kind != nil {
		q += fmt.Sprintf(` AND kind = $%d`, next)
		args = append(args, *p.Kind)
		next++
	}
	if p.Scope != nil {
		q += fmt.Sprintf(` AND scope = $%d`, next)
		args = append(args, *p.Scope)
		next++
	}
	q += ` ORDER BY date DESC NULLS LAST, created_at DESC`
	return r.query(ctx, q, args...)
}

// ListActiveForGrounding returns the items the context aggregators fold in:
// status='active', not expired as of `asOf`, with standing items always included
// and recommendations narrowed to [recFrom, recTo]. NeedsReview is set on items
// whose review_at is on or before `asOf`.
func (r *Repo) ListActiveForGrounding(ctx context.Context, asOf, recFrom, recTo string) ([]*Memory, error) {
	const q = `SELECT ` + selectCols + ` FROM coach_memory
        WHERE status = 'active'
          AND (expires_at IS NULL OR expires_at >= $1::date)
          AND (kind <> 'recommendation' OR (date >= $2::date AND date <= $3::date))
        ORDER BY date DESC NULLS LAST, created_at DESC`
	out, err := r.query(ctx, q, asOf, recFrom, recTo)
	if err != nil {
		return nil, err
	}
	for _, m := range out {
		if m.ReviewAt != nil && *m.ReviewAt <= asOf {
			m.NeedsReview = true
		}
	}
	return out, nil
}

// PatchParams carries the lifecycle-only mutable fields. nil pointer + Clear=false
// leaves a field unchanged; a non-nil pointer sets it; nil + Clear=true clears the
// (nullable) date to NULL. Status has no clear flag (NOT NULL).
type PatchParams struct {
	ReviewAt       *string
	ClearReviewAt  bool
	ExpiresAt      *string
	ClearExpiresAt bool
	Status         *string
}

// HasUpdates reports whether at least one lifecycle field is being changed.
func (p PatchParams) HasUpdates() bool {
	return p.ReviewAt != nil || p.ClearReviewAt ||
		p.ExpiresAt != nil || p.ClearExpiresAt ||
		p.Status != nil
}

// Patch applies the lifecycle update in place (preserving created_at). Content
// fields (text/kind/scope/date) are never touched here — corrections are delete
// + re-log. Returns ErrNotFound if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.ClearReviewAt {
		sets = append(sets, "review_at = NULL")
	} else if p.ReviewAt != nil {
		sets = append(sets, fmt.Sprintf("review_at = $%d::date", next))
		args = append(args, *p.ReviewAt)
		next++
	}
	if p.ClearExpiresAt {
		sets = append(sets, "expires_at = NULL")
	} else if p.ExpiresAt != nil {
		sets = append(sets, fmt.Sprintf("expires_at = $%d::date", next))
		args = append(args, *p.ExpiresAt)
		next++
	}
	if p.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", next))
		args = append(args, *p.Status)
		next++
	}
	if len(sets) == 1 {
		// Nothing to update — confirm existence so the caller distinguishes
		// "noop on existing" from "404 on missing".
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE coach_memory SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch coach memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) query(ctx context.Context, q string, args ...any) ([]*Memory, error) {
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list coach memory: %w", err)
	}
	defer rows.Close()
	var out []*Memory
	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// scopeArg converts the optional typed scope into the *string the driver binds
// (nil → SQL NULL).
func scopeArg(s *Scope) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(s scanner) (*Memory, error) {
	var m Memory
	if err := scanInto(s, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func scanInto(s scanner, m *Memory) error {
	var (
		kindStr   string
		statusStr string
		scopeStr  *string
	)
	err := s.Scan(
		&m.ID, &kindStr, &m.Text, &m.Reason, &scopeStr,
		&m.Date, &m.ExpiresAt, &m.ReviewAt,
		&statusStr, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("scan coach memory: %w", err)
	}
	m.Kind = Kind(kindStr)
	m.Status = Status(statusStr)
	if scopeStr != nil {
		sc := Scope(*scopeStr)
		m.Scope = &sc
	}
	return nil
}
