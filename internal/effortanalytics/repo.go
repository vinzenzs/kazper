package effortanalytics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// Repo persists best-effort records and serves the windowed curve.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// Replace overwrites a workout's best-effort rows in one transaction: delete the
// workout's existing rows, then insert the freshly computed set. Replace-on-
// repost keeps a re-sync idempotent. achievedAt (the workout's started_at) is
// stamped on every row so the curve can name the day an effort came from.
func (r *Repo) Replace(ctx context.Context, workoutID uuid.UUID, achievedAt time.Time, recs []Record) error {
	return store.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM workout_best_efforts WHERE workout_id = $1`, workoutID); err != nil {
			return fmt.Errorf("clear best efforts: %w", err)
		}
		for _, rec := range recs {
			if _, err := tx.Exec(ctx, `
                INSERT INTO workout_best_efforts (id, workout_id, metric, duration_s, value, achieved_at, kj_tier)
                VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				uuid.New(), workoutID, string(rec.Metric), rec.DurationS, rec.Value, achievedAt, rec.KJTier,
			); err != nil {
				return fmt.Errorf("insert best effort: %w", err)
			}
		}
		return nil
	})
}

// Curve returns, per duration, the best (max) value of `metric` across completed
// workouts of `sport` whose started_at falls in [from, to], plus the contributing
// workout and day. DISTINCT ON picks the top value per duration. Pinned to
// kj_tier = 0 — the FRESH ladder — so durability tiers never leak into the
// power/CP/profile curves (add-durability-analysis). The sport predicate keeps a
// run's running-power rows out of bike windows and a bike's speed rows out of
// run/swim windows; multisport workouts match no sport-scoped window
// (fix-effort-ladder-sport-scoping).
func (r *Repo) Curve(ctx context.Context, from, to time.Time, metric Metric, sport string) ([]CurvePoint, error) {
	const q = `
        SELECT DISTINCT ON (be.duration_s)
            be.duration_s, be.value, be.workout_id, be.achieved_at
        FROM workout_best_efforts be
        JOIN workouts w ON w.id = be.workout_id
        WHERE be.metric = $1
          AND be.kj_tier = 0
          AND w.status = 'completed'
          AND w.sport = $4
          AND w.started_at >= $2
          AND w.started_at <= $3
        ORDER BY be.duration_s, be.value DESC`
	rows, err := r.pool.Query(ctx, q, string(metric), from, to, sport)
	if err != nil {
		return nil, fmt.Errorf("query curve: %w", err)
	}
	defer rows.Close()

	var out []CurvePoint
	for rows.Next() {
		var (
			dur       int
			value     float64
			workoutID uuid.UUID
			achieved  time.Time
		)
		if err := rows.Scan(&dur, &value, &workoutID, &achieved); err != nil {
			return nil, fmt.Errorf("scan curve point: %w", err)
		}
		out = append(out, CurvePoint{
			DurationS: dur,
			Value:     value,
			WorkoutID: workoutID.String(),
			Date:      achieved.Format("2006-01-02"),
		})
	}
	return out, rows.Err()
}

// TierBest is one windowed best at a (duration, kj_tier): the MAX watts and the
// contributing workout/day. Tier 0 is the fresh ladder.
type TierBest struct {
	DurationS int
	KJTier    int
	Watts     float64
	WorkoutID string
	Date      string
}

// DurabilityBests returns, per (duration, kj_tier) over the window, the best
// (max) POWER value with its contributing workout — the fresh (tier 0) and every
// tiered best in one pass, restricted to the durability durations. Compute-on-
// read over stored rows only (no stream scans).
func (r *Repo) DurabilityBests(ctx context.Context, from, to time.Time) ([]TierBest, error) {
	const q = `
        SELECT DISTINCT ON (be.duration_s, be.kj_tier)
            be.duration_s, be.kj_tier, be.value, be.workout_id, be.achieved_at
        FROM workout_best_efforts be
        JOIN workouts w ON w.id = be.workout_id
        WHERE be.metric = 'power'
          AND be.duration_s = ANY($1)
          AND w.status = 'completed'
          AND w.sport = 'bike'
          AND w.started_at >= $2
          AND w.started_at <= $3
        ORDER BY be.duration_s, be.kj_tier, be.value DESC`
	rows, err := r.pool.Query(ctx, q, DurabilityDurations, from, to)
	if err != nil {
		return nil, fmt.Errorf("query durability: %w", err)
	}
	defer rows.Close()

	var out []TierBest
	for rows.Next() {
		var (
			t         TierBest
			workoutID uuid.UUID
			achieved  time.Time
		)
		if err := rows.Scan(&t.DurationS, &t.KJTier, &t.Watts, &workoutID, &achieved); err != nil {
			return nil, fmt.Errorf("scan durability row: %w", err)
		}
		t.WorkoutID = workoutID.String()
		t.Date = achieved.Format("2006-01-02")
		out = append(out, t)
	}
	return out, rows.Err()
}
