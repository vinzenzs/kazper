package locations

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no period exists for an id.
var ErrNotFound = errors.New("location period not found")

// Repo persists location periods against a pgxpool.Pool or a pgx.Tx.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo { return &Repo{q: q} }

// Dates are stored as DATE and read back as text so they never acquire a
// spurious time or timezone on the round trip — a travel period is calendar
// data, not an instant.
const selectCols = `id, to_char(start_date, 'YYYY-MM-DD'), to_char(end_date, 'YYYY-MM-DD'),
	name, lat, lon, note, created_at, updated_at`

// Insert creates a period and returns it.
func (r *Repo) Insert(ctx context.Context, p *Period) (*Period, error) {
	id := uuid.New()
	_, err := r.q.Exec(ctx,
		`INSERT INTO location_periods (id, start_date, end_date, name, lat, lon, note)
		 VALUES ($1, $2::date, $3::date, $4, $5, $6, $7)`,
		id, p.StartDate, p.EndDate, p.Name, p.Lat, p.Lon, p.Note,
	)
	if err != nil {
		return nil, fmt.Errorf("insert location period: %w", err)
	}
	return r.GetByID(ctx, id)
}

// GetByID returns the period, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Period, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM location_periods WHERE id = $1`, id)
	p, err := scanPeriod(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan location period: %w", err)
	}
	return p, nil
}

// ListOverlapping returns every period intersecting the inclusive [from, to]
// window, ascending by start_date. Overlap, not containment: a camp that starts
// before the window and ends inside it is still where the athlete was.
func (r *Repo) ListOverlapping(ctx context.Context, from, to string) ([]*Period, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM location_periods
		 WHERE start_date <= $2::date AND end_date >= $1::date
		 ORDER BY start_date ASC, created_at ASC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list location periods: %w", err)
	}
	defer rows.Close()

	out := []*Period{}
	for rows.Next() {
		p, err := scanPeriod(rows)
		if err != nil {
			return nil, fmt.Errorf("scan location period: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CoveringOn returns the period covering `date` with the latest start_date, or
// nil when none does. Overlaps are legal, so the tie-break is the rule that
// makes resolution total: a weekend trip logged inside a training camp wins for
// its dates (the macrocycle/public-feed rule). created_at breaks an exact
// start_date tie — the more recently logged period is the more recent intent.
func (r *Repo) CoveringOn(ctx context.Context, date string) (*Period, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM location_periods
		 WHERE start_date <= $1::date AND end_date >= $1::date
		 ORDER BY start_date DESC, created_at DESC
		 LIMIT 1`, date)
	p, err := scanPeriod(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // no covering period is a normal state, not an error
		}
		return nil, fmt.Errorf("scan covering location period: %w", err)
	}
	return p, nil
}

// Delete removes a period. ErrNotFound when the id doesn't exist.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM location_periods WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete location period: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPeriod(s scanner) (*Period, error) {
	var p Period
	if err := s.Scan(&p.ID, &p.StartDate, &p.EndDate, &p.Name, &p.Lat, &p.Lon,
		&p.Note, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}
