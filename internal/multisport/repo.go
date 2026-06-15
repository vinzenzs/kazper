package multisport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no multisport template exists for an id.
var ErrNotFound = errors.New("multisport template not found")

// Repo persists multisport templates.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, name, segments, created_at, updated_at`

// Create inserts a template and returns the persisted row (id + timestamps).
func (r *Repo) Create(ctx context.Context, t *Template) (*Template, error) {
	segJSON, err := json.Marshal(t.Segments)
	if err != nil {
		return nil, fmt.Errorf("marshal segments: %w", err)
	}
	row := r.q.QueryRow(ctx, `
        INSERT INTO multisport_templates (name, segments, created_at, updated_at)
        VALUES ($1, $2, now(), now())
        RETURNING `+selectCols,
		t.Name, segJSON,
	)
	return scanTemplate(row)
}

// GetByID returns the template for an id, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (*Template, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM multisport_templates WHERE id = $1`, id)
	return scanTemplate(row)
}

// List returns templates newest first.
func (r *Repo) List(ctx context.Context) ([]*Template, error) {
	rows, err := r.q.Query(ctx, `SELECT `+selectCols+` FROM multisport_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list multisport templates: %w", err)
	}
	defer rows.Close()
	var out []*Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a template. Returns ErrNotFound if none existed.
func (r *Repo) Delete(ctx context.Context, id string) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM multisport_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete multisport template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTemplate(s scanner) (*Template, error) {
	var (
		t       Template
		segsRaw []byte
	)
	err := s.Scan(&t.ID, &t.Name, &segsRaw, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan multisport template: %w", err)
	}
	if len(segsRaw) > 0 {
		if err := json.Unmarshal(segsRaw, &t.Segments); err != nil {
			return nil, fmt.Errorf("scan multisport template segments: %w", err)
		}
	}
	return &t, nil
}
