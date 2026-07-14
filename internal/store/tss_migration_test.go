package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestMigrate_TSSSourceBackfill verifies migration 055: existing workouts rows
// with a tss back-fill their new tss_source by source (garmin→'garmin',
// else→'manual'), NULL-tss rows stay NULL, and the column + CHECKs exist.
func TestMigrate_TSSSourceBackfill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed migration test in short mode")
	}
	ctx := context.Background()
	dsn := startPostgres(t)

	m, err := newMigrator(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })

	// Migrate up to 054 (before tss_source exists), then seed rows.
	require.NoError(t, m.Migrate(54), "migrate to 054")

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	seed := func(source string, tss *float64) string {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO workouts (id, source, sport, status, started_at, ended_at, created_at, updated_at, tss)
			VALUES (gen_random_uuid(), $1, 'bike', 'completed', now() - interval '2 hours', now() - interval '1 hour', now(), now(), $2)
			RETURNING id::text`, source, tss).Scan(&id)
		require.NoError(t, err)
		return id
	}
	tss80 := 80.0
	garminID := seed("garmin", &tss80)
	manualID := seed("manual", &tss80)
	nullID := seed("garmin", nil)

	// Apply 055 (adds the column, back-fills, then the pairing CHECK).
	require.NoError(t, m.Migrate(55), "migrate to 055")

	sourceOf := func(id string) *string {
		var s *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT tss_source FROM workouts WHERE id = $1::uuid`, id).Scan(&s))
		return s
	}
	require.NotNil(t, sourceOf(garminID))
	require.Equal(t, "garmin", *sourceOf(garminID))
	require.NotNil(t, sourceOf(manualID))
	require.Equal(t, "manual", *sourceOf(manualID))
	require.Nil(t, sourceOf(nullID), "NULL-tss row keeps NULL tss_source")

	// Column + both CHECK constraints exist.
	var checks int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conrelid = 'workouts'::regclass AND contype = 'c'
		  AND conname IN ('workouts_tss_source_check', 'workouts_tss_source_pairing')`).Scan(&checks))
	require.Equal(t, 2, checks)
}
