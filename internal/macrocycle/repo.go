package macrocycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrMacrocycleNotFound is returned when no macrocycle row matches a lookup.
var ErrMacrocycleNotFound = errors.New("macrocycle not found")

// Repo persists macrocycles rows and reads their member phases.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// scanner is the minimal interface both pgx.Row and pgx.Rows satisfy.
type scanner interface {
	Scan(dest ...any) error
}

// Column projection that LEFT JOINs races so RaceName is populated in one round
// trip. The trailing `name` is from the races table (NULL when unanchored).
const macroSelectCols = `
    m.id, m.name, m.start_date, m.end_date,
    m.race_id, m.methodology, m.notes,
    m.created_at, m.updated_at,
    r.name
`

const macroFromJoin = `
    FROM macrocycles m
    LEFT JOIN races r ON r.id = m.race_id
`

// Insert creates a macrocycle row. Caller validates; the repo does no semantic
// checks beyond what the DB CHECK/FK constraints enforce.
func (r *Repo) Insert(ctx context.Context, m *Macrocycle) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO macrocycles
            (id, name, start_date, end_date, race_id, methodology, notes, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
    `
	if _, err := r.q.Exec(ctx, q,
		m.ID, m.Name, m.StartDate, m.EndDate, m.RaceID, m.Methodology, m.Notes, now,
	); err != nil {
		return fmt.Errorf("insert macrocycle: %w", err)
	}
	m.CreatedAt = now
	m.UpdatedAt = now
	return nil
}

// GetByID returns the macrocycle (with RaceName resolved) plus its ordered
// member phases. Phases is non-nil (possibly empty) so the by-id response
// always carries a `phases` array.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Macrocycle, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+macroSelectCols+macroFromJoin+` WHERE m.id = $1`, id)
	m, err := scanMacrocycle(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMacrocycleNotFound
		}
		return nil, fmt.Errorf("scan macrocycle: %w", err)
	}
	phases, err := r.memberPhases(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Phases = phases
	return m, nil
}

// List returns every macrocycle ordered by start_date DESC (seasons are few;
// the list omits the nested member phases).
func (r *Repo) List(ctx context.Context) ([]*Macrocycle, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+macroSelectCols+macroFromJoin+` ORDER BY m.start_date DESC, m.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list macrocycles: %w", err)
	}
	defer rows.Close()
	var out []*Macrocycle
	for rows.Next() {
		m, err := scanMacrocycle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// memberPhases reads the lite projection of training_phases rows linked to the
// macrocycle, ordered by macrocycle_ordinal (nulls last) then start_date.
func (r *Repo) memberPhases(ctx context.Context, id uuid.UUID) ([]*MemberPhase, error) {
	rows, err := r.q.Query(ctx, `
        SELECT id, name, type, start_date, end_date,
               macrocycle_ordinal, target_weekly_tss, target_weekly_hours
          FROM training_phases
         WHERE macrocycle_id = $1
         ORDER BY macrocycle_ordinal ASC NULLS LAST, start_date ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("member phases: %w", err)
	}
	defer rows.Close()
	out := []*MemberPhase{}
	for rows.Next() {
		var mp MemberPhase
		if err := rows.Scan(
			&mp.ID, &mp.Name, &mp.Type, &mp.StartDate, &mp.EndDate,
			&mp.MacrocycleOrdinal, &mp.TargetWeeklyTSS, &mp.TargetWeeklyHours,
		); err != nil {
			return nil, fmt.Errorf("scan member phase: %w", err)
		}
		out = append(out, &mp)
	}
	return out, rows.Err()
}

// Patch applies a partial update. Returns ErrMacrocycleNotFound on a miss.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchInput) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", next))
		args = append(args, *p.Name)
		next++
	}
	if p.StartDate != nil {
		sets = append(sets, fmt.Sprintf("start_date = $%d", next))
		args = append(args, *p.StartDate)
		next++
	}
	if p.EndDate != nil {
		sets = append(sets, fmt.Sprintf("end_date = $%d", next))
		args = append(args, *p.EndDate)
		next++
	}
	if p.ClearRaceID {
		sets = append(sets, "race_id = NULL")
	} else if p.RaceID != nil {
		sets = append(sets, fmt.Sprintf("race_id = $%d", next))
		args = append(args, *p.RaceID)
		next++
	}
	if p.Methodology != nil {
		sets = append(sets, fmt.Sprintf("methodology = $%d", next))
		args = append(args, *p.Methodology)
		next++
	}
	if p.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", next))
		args = append(args, *p.Notes)
		next++
	}
	if len(sets) == 1 {
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE macrocycles SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch macrocycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMacrocycleNotFound
	}
	return nil
}

// Delete removes a macrocycle. Member phases' macrocycle_id is set NULL by the
// FK (ON DELETE SET NULL) — the phases survive. Returns ErrMacrocycleNotFound
// if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM macrocycles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete macrocycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMacrocycleNotFound
	}
	return nil
}

// MacrocycleExists reports whether a macrocycle row exists. Used by the
// training-phases service for FK pre-validation (cross-injected).
func (r *Repo) MacrocycleExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	if err := r.q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM macrocycles WHERE id = $1)`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("macrocycle exists: %w", err)
	}
	return exists, nil
}

// CoveringFor returns the most-recently-updated macrocycle covering `date`
// (inclusive), with its race anchor resolved and its member-phase count, or
// ErrMacrocycleNotFound if none. Used by the coach-context training bundle.
func (r *Repo) CoveringFor(ctx context.Context, date time.Time) (*Covering, error) {
	row := r.q.QueryRow(ctx, `
        SELECT m.id, m.name, m.start_date, m.end_date,
               m.race_id, r.name, r.race_date,
               (SELECT count(*) FROM training_phases p WHERE p.macrocycle_id = m.id)
          FROM macrocycles m
          LEFT JOIN races r ON r.id = m.race_id
         WHERE m.start_date <= $1 AND m.end_date >= $1
         ORDER BY m.updated_at DESC
         LIMIT 1`, date)
	var c Covering
	if err := row.Scan(
		&c.ID, &c.Name, &c.StartDate, &c.EndDate,
		&c.RaceID, &c.RaceName, &c.RaceDate, &c.TotalPeriods,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMacrocycleNotFound
		}
		return nil, fmt.Errorf("covering macrocycle: %w", err)
	}
	return &c, nil
}

// scanMacrocycle scans one row from the macroSelectCols+macroFromJoin
// projection (macrocycles columns + the joined race name as the last column,
// NULL when unanchored).
func scanMacrocycle(s scanner) (*Macrocycle, error) {
	var m Macrocycle
	if err := s.Scan(
		&m.ID, &m.Name, &m.StartDate, &m.EndDate,
		&m.RaceID, &m.Methodology, &m.Notes,
		&m.CreatedAt, &m.UpdatedAt,
		&m.RaceName,
	); err != nil {
		return nil, err
	}
	return &m, nil
}
