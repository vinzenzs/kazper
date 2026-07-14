package athleteconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// singletonID is the fixed primary key of the one allowed athlete_config row.
const singletonID = "00000000-0000-0000-0000-000000000001"

// Repo persists the athlete_config singleton row.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `
    ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
    threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
    hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
    power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max,
    created_at, updated_at
`

// Get returns the config row, or (nil, nil) if no row exists yet. The nil-row
// signal is distinct from any DB error so the handler can return
// {"athlete_config": null} straightforwardly.
func (r *Repo) Get(ctx context.Context) (*AthleteConfig, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM athlete_config WHERE id = $1`, singletonID)
	cfg, err := scanConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan athlete config: %w", err)
	}
	return cfg, nil
}

// Upsert writes the singleton row, replacing all field values with what's on
// cfg. Absent fields (nil pointers) overwrite to NULL — full-replace PUT
// semantics, matching PUT /goals.
func (r *Repo) Upsert(ctx context.Context, cfg *AthleteConfig) error {
	const q = `
        INSERT INTO athlete_config (
            id,
            ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
            threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
            hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
            power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max,
            updated_at
        ) VALUES (
            $1,
            $2, $3, $4, $5,
            $6, $7,
            $8, $9, $10, $11, $12,
            $13, $14, $15, $16, $17,
            now()
        )
        ON CONFLICT (id) DO UPDATE SET
            ftp_watts                        = EXCLUDED.ftp_watts,
            threshold_hr                     = EXCLUDED.threshold_hr,
            lactate_threshold_hr             = EXCLUDED.lactate_threshold_hr,
            max_hr                           = EXCLUDED.max_hr,
            threshold_pace_sec_per_km        = EXCLUDED.threshold_pace_sec_per_km,
            threshold_swim_pace_sec_per_100m = EXCLUDED.threshold_swim_pace_sec_per_100m,
            hr_zone_1_max                    = EXCLUDED.hr_zone_1_max,
            hr_zone_2_max                    = EXCLUDED.hr_zone_2_max,
            hr_zone_3_max                    = EXCLUDED.hr_zone_3_max,
            hr_zone_4_max                    = EXCLUDED.hr_zone_4_max,
            hr_zone_5_max                    = EXCLUDED.hr_zone_5_max,
            power_zone_1_max                 = EXCLUDED.power_zone_1_max,
            power_zone_2_max                 = EXCLUDED.power_zone_2_max,
            power_zone_3_max                 = EXCLUDED.power_zone_3_max,
            power_zone_4_max                 = EXCLUDED.power_zone_4_max,
            power_zone_5_max                 = EXCLUDED.power_zone_5_max,
            updated_at                       = now()
    `
	_, err := r.q.Exec(ctx, q,
		singletonID,
		cfg.FtpWatts, cfg.ThresholdHR, cfg.LactateThresholdHR, cfg.MaxHR,
		cfg.ThresholdPaceSecPerKm, cfg.ThresholdSwimPaceSecPer100m,
		cfg.HRZone1Max, cfg.HRZone2Max, cfg.HRZone3Max, cfg.HRZone4Max, cfg.HRZone5Max,
		cfg.PowerZone1Max, cfg.PowerZone2Max, cfg.PowerZone3Max, cfg.PowerZone4Max, cfg.PowerZone5Max,
	)
	if err != nil {
		return fmt.Errorf("upsert athlete config: %w", err)
	}
	return nil
}

// historyPhysiologyCols is the 16 physiology columns of athlete_config_history,
// in the fixed order used by every history query/scan.
const historyPhysiologyCols = `
    ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
    threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
    hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
    power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max
`

// physiologyArgs returns the 16 field pointers of cfg in historyPhysiologyCols
// order, for binding to an INSERT/UPSERT.
func physiologyArgs(cfg *AthleteConfig) []any {
	return []any{
		cfg.FtpWatts, cfg.ThresholdHR, cfg.LactateThresholdHR, cfg.MaxHR,
		cfg.ThresholdPaceSecPerKm, cfg.ThresholdSwimPaceSecPer100m,
		cfg.HRZone1Max, cfg.HRZone2Max, cfg.HRZone3Max, cfg.HRZone4Max, cfg.HRZone5Max,
		cfg.PowerZone1Max, cfg.PowerZone2Max, cfg.PowerZone3Max, cfg.PowerZone4Max, cfg.PowerZone5Max,
	}
}

// UpsertSnapshot inserts or replaces the history row for effective_from with the
// full physiology state (same-day replace on the PK).
func (r *Repo) UpsertSnapshot(ctx context.Context, effectiveFrom time.Time, cfg *AthleteConfig) error {
	const q = `
        INSERT INTO athlete_config_history (
            effective_from,
            ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
            threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
            hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
            power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
        ON CONFLICT (effective_from) DO UPDATE SET
            ftp_watts                        = EXCLUDED.ftp_watts,
            threshold_hr                     = EXCLUDED.threshold_hr,
            lactate_threshold_hr             = EXCLUDED.lactate_threshold_hr,
            max_hr                           = EXCLUDED.max_hr,
            threshold_pace_sec_per_km        = EXCLUDED.threshold_pace_sec_per_km,
            threshold_swim_pace_sec_per_100m = EXCLUDED.threshold_swim_pace_sec_per_100m,
            hr_zone_1_max                    = EXCLUDED.hr_zone_1_max,
            hr_zone_2_max                    = EXCLUDED.hr_zone_2_max,
            hr_zone_3_max                    = EXCLUDED.hr_zone_3_max,
            hr_zone_4_max                    = EXCLUDED.hr_zone_4_max,
            hr_zone_5_max                    = EXCLUDED.hr_zone_5_max,
            power_zone_1_max                 = EXCLUDED.power_zone_1_max,
            power_zone_2_max                 = EXCLUDED.power_zone_2_max,
            power_zone_3_max                 = EXCLUDED.power_zone_3_max,
            power_zone_4_max                 = EXCLUDED.power_zone_4_max,
            power_zone_5_max                 = EXCLUDED.power_zone_5_max,
            updated_at                       = now()`
	args := append([]any{effectiveFrom}, physiologyArgs(cfg)...)
	if _, err := r.q.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("upsert config snapshot: %w", err)
	}
	return nil
}

// DeleteSnapshot removes the history row for effective_from (no-op if absent).
func (r *Repo) DeleteSnapshot(ctx context.Context, effectiveFrom time.Time) error {
	if _, err := r.q.Exec(ctx, `DELETE FROM athlete_config_history WHERE effective_from = $1`, effectiveFrom); err != nil {
		return fmt.Errorf("delete config snapshot: %w", err)
	}
	return nil
}

// LatestBefore returns the newest snapshot strictly before date, or (nil, nil)
// when there is none. Used to collapse a same-day revert to the prior state.
func (r *Repo) LatestBefore(ctx context.Context, date time.Time) (*ThresholdSnapshot, error) {
	row := r.q.QueryRow(ctx,
		`SELECT effective_from, `+historyPhysiologyCols+`, created_at, updated_at
         FROM athlete_config_history WHERE effective_from < $1
         ORDER BY effective_from DESC LIMIT 1`, date)
	return scanSnapshotOrNil(row)
}

// AsOf returns the snapshot in effect on date (latest with effective_from <=
// date), or (nil, nil) when history is empty / date precedes all snapshots.
func (r *Repo) AsOf(ctx context.Context, date time.Time) (*ThresholdSnapshot, error) {
	row := r.q.QueryRow(ctx,
		`SELECT effective_from, `+historyPhysiologyCols+`, created_at, updated_at
         FROM athlete_config_history WHERE effective_from <= $1
         ORDER BY effective_from DESC LIMIT 1`, date)
	return scanSnapshotOrNil(row)
}

// ListHistory returns snapshots ascending by effective_from, optionally bounded
// by inclusive from/to (nil = unbounded on that side).
func (r *Repo) ListHistory(ctx context.Context, from, to *time.Time) ([]*ThresholdSnapshot, error) {
	rows, err := r.q.Query(ctx,
		`SELECT effective_from, `+historyPhysiologyCols+`, created_at, updated_at
         FROM athlete_config_history
         WHERE ($1::date IS NULL OR effective_from >= $1)
           AND ($2::date IS NULL OR effective_from <= $2)
         ORDER BY effective_from ASC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list config history: %w", err)
	}
	defer rows.Close()
	out := []*ThresholdSnapshot{}
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func scanSnapshotOrNil(s scanner) (*ThresholdSnapshot, error) {
	snap, err := scanSnapshot(s)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return snap, nil
}

func scanSnapshot(s scanner) (*ThresholdSnapshot, error) {
	var snap ThresholdSnapshot
	var effectiveFrom time.Time
	c := &snap.AthleteConfig
	if err := s.Scan(
		&effectiveFrom,
		&c.FtpWatts, &c.ThresholdHR, &c.LactateThresholdHR, &c.MaxHR,
		&c.ThresholdPaceSecPerKm, &c.ThresholdSwimPaceSecPer100m,
		&c.HRZone1Max, &c.HRZone2Max, &c.HRZone3Max, &c.HRZone4Max, &c.HRZone5Max,
		&c.PowerZone1Max, &c.PowerZone2Max, &c.PowerZone3Max, &c.PowerZone4Max, &c.PowerZone5Max,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	snap.EffectiveFrom = effectiveFrom.Format(dateLayout)
	return &snap, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConfig(s scanner) (*AthleteConfig, error) {
	var cfg AthleteConfig
	err := s.Scan(
		&cfg.FtpWatts, &cfg.ThresholdHR, &cfg.LactateThresholdHR, &cfg.MaxHR,
		&cfg.ThresholdPaceSecPerKm, &cfg.ThresholdSwimPaceSecPer100m,
		&cfg.HRZone1Max, &cfg.HRZone2Max, &cfg.HRZone3Max, &cfg.HRZone4Max, &cfg.HRZone5Max,
		&cfg.PowerZone1Max, &cfg.PowerZone2Max, &cfg.PowerZone3Max, &cfg.PowerZone4Max, &cfg.PowerZone5Max,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
