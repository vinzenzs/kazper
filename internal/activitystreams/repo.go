package activitystreams

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// Repo persists raw workout streams. Samples cross the DB boundary as float32
// (the column is REAL[] / float4[]); the service works in float64.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// Replace atomically swaps the workout's stored streams for the provided set:
// every existing row for the workout is deleted, then one row per non-empty
// series is inserted at 1 Hz. Returns the number of streams stored. An empty set
// still clears any prior streams (a re-post with fewer series drops the rest).
func (r *Repo) Replace(ctx context.Context, workoutID uuid.UUID, series map[StreamType][]float64) (int, error) {
	stored := 0
	err := store.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM workout_streams WHERE workout_id = $1`, workoutID); err != nil {
			return err
		}
		for _, st := range []StreamType{StreamPower, StreamSpeed, StreamHeartRate} {
			samples := series[st]
			if len(samples) == 0 {
				continue
			}
			f32 := toFloat32(samples)
			if _, err := tx.Exec(ctx,
				`INSERT INTO workout_streams (workout_id, stream_type, samples, sample_rate_hz, sample_count)
				 VALUES ($1, $2, $3, 1, $4)`,
				workoutID, string(st), f32, len(f32),
			); err != nil {
				return err
			}
			stored++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return stored, nil
}

// LoadForWorkout returns every stored stream for the workout keyed by type,
// converted to float64, plus the sample rate (defaults to 1 Hz). An empty map
// (with sampleRate 0) means no streams are stored.
func (r *Repo) LoadForWorkout(ctx context.Context, workoutID uuid.UUID) (map[StreamType][]float64, int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT stream_type, samples, sample_rate_hz FROM workout_streams WHERE workout_id = $1`,
		workoutID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := map[StreamType][]float64{}
	sampleRate := 0
	for rows.Next() {
		var st string
		var samples []float32
		var rate int
		if err := rows.Scan(&st, &samples, &rate); err != nil {
			return nil, 0, err
		}
		out[StreamType(st)] = toFloat64(samples)
		sampleRate = rate
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, sampleRate, nil
}

func toFloat32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

func toFloat64(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}
