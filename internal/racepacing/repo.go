package racepacing

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrOverrideNotFound is returned by DeleteOverride when no override exists for
// the (race_id, ordinal) key.
var ErrOverrideNotFound = errors.New("override not found")

// Repo persists per-leg pacing overrides. Works against *pgxpool.Pool or pgx.Tx.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const overrideCols = `race_id, ordinal, target_power_low_w, target_power_high_w,
	target_pace_low_sec_per_km, target_pace_high_sec_per_km,
	target_pace_low_sec_per_100m, target_pace_high_sec_per_100m,
	note, created_at, updated_at`

// UpsertOverride full-replaces the override for (race_id, ordinal). Exactly one
// unit family on o must be populated (the caller/service validates first; the DB
// CHECKs are the backstop).
func (r *Repo) UpsertOverride(ctx context.Context, o *Override) error {
	_, err := r.q.Exec(ctx, `
        INSERT INTO race_leg_pacing_overrides (
            race_id, ordinal,
            target_power_low_w, target_power_high_w,
            target_pace_low_sec_per_km, target_pace_high_sec_per_km,
            target_pace_low_sec_per_100m, target_pace_high_sec_per_100m,
            note, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
        ON CONFLICT (race_id, ordinal) DO UPDATE SET
            target_power_low_w            = EXCLUDED.target_power_low_w,
            target_power_high_w           = EXCLUDED.target_power_high_w,
            target_pace_low_sec_per_km    = EXCLUDED.target_pace_low_sec_per_km,
            target_pace_high_sec_per_km   = EXCLUDED.target_pace_high_sec_per_km,
            target_pace_low_sec_per_100m  = EXCLUDED.target_pace_low_sec_per_100m,
            target_pace_high_sec_per_100m = EXCLUDED.target_pace_high_sec_per_100m,
            note                          = EXCLUDED.note,
            updated_at                    = now()`,
		o.RaceID, o.Ordinal,
		o.TargetPowerLowW, o.TargetPowerHighW,
		o.TargetPaceLowSecPerKM, o.TargetPaceHighSecPerKM,
		o.TargetPaceLowSecPer100m, o.TargetPaceHighSecPer100m,
		o.Note,
	)
	return err
}

// GetOverridesForRace returns all overrides for a race, keyed by ordinal.
func (r *Repo) GetOverridesForRace(ctx context.Context, raceID uuid.UUID) (map[int]*Override, error) {
	rows, err := r.q.Query(ctx, `SELECT `+overrideCols+` FROM race_leg_pacing_overrides WHERE race_id = $1`, raceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]*Override{}
	for rows.Next() {
		var o Override
		if err := rows.Scan(
			&o.RaceID, &o.Ordinal,
			&o.TargetPowerLowW, &o.TargetPowerHighW,
			&o.TargetPaceLowSecPerKM, &o.TargetPaceHighSecPerKM,
			&o.TargetPaceLowSecPer100m, &o.TargetPaceHighSecPer100m,
			&o.Note, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out[o.Ordinal] = &o
	}
	return out, rows.Err()
}

// DeleteOverride removes the override for (race_id, ordinal). ErrOverrideNotFound
// when none existed.
func (r *Repo) DeleteOverride(ctx context.Context, raceID uuid.UUID, ordinal int) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM race_leg_pacing_overrides WHERE race_id = $1 AND ordinal = $2`, raceID, ordinal)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOverrideNotFound
	}
	return nil
}
