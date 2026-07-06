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
                INSERT INTO workout_best_efforts (id, workout_id, metric, duration_s, value, achieved_at)
                VALUES ($1, $2, $3, $4, $5, $6)`,
				uuid.New(), workoutID, string(rec.Metric), rec.DurationS, rec.Value, achievedAt,
			); err != nil {
				return fmt.Errorf("insert best effort: %w", err)
			}
		}
		return nil
	})
}

// Curve returns, per duration, the best (max) value of `metric` across completed
// workouts whose started_at falls in [from, to], plus the contributing workout
// and day. DISTINCT ON picks the top value per duration.
func (r *Repo) Curve(ctx context.Context, from, to time.Time, metric Metric) ([]CurvePoint, error) {
	const q = `
        SELECT DISTINCT ON (be.duration_s)
            be.duration_s, be.value, be.workout_id, be.achieved_at
        FROM workout_best_efforts be
        JOIN workouts w ON w.id = be.workout_id
        WHERE be.metric = $1
          AND w.status = 'completed'
          AND w.started_at >= $2
          AND w.started_at <= $3
        ORDER BY be.duration_s, be.value DESC`
	rows, err := r.pool.Query(ctx, q, string(metric), from, to)
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
