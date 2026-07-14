package store

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var migrateTestConfigOnce sync.Once

func disableRyuk() {
	migrateTestConfigOnce.Do(func() {
		if _, ok := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED"); !ok {
			os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
		}
	})
}

func startPostgres(t *testing.T) string {
	t.Helper()
	disableRyuk()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("nutrition"),
		tcpostgres.WithUsername("nutrition"),
		tcpostgres.WithPassword("nutrition"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get connection string")
	return dsn
}

// TestMigrate_UpDownUp verifies that the migration set can roll forward, all
// the way back down, and forward again on the same database without errors.
// This catches missing IF EXISTS guards in down migrations and ordering issues
// (e.g. a column drop that runs before the dependent constraint is restored).
func TestMigrate_UpDownUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed migration cycle in short mode")
	}
	dsn := startPostgres(t)

	require.NoError(t, Migrate(dsn), "initial Up")

	m, err := newMigrator(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			t.Logf("migrator source close: %v", srcErr)
		}
		if dbErr != nil {
			t.Logf("migrator db close: %v", dbErr)
		}
	})

	require.NoError(t, m.Down(), "Down")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("re-Up after Down: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	for _, table := range []string{"products", "meal_entries", "idempotency_records", "product_components", "nutrition_goals"} {
		var exists bool
		err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`,
			table,
		).Scan(&exists)
		require.NoError(t, err, "lookup %s", table)
		require.True(t, exists, "table %s should exist after Up", table)
	}

	for _, col := range []string{
		"iron_mg_per_100g", "calcium_mg_per_100g", "vitamin_d_mcg_per_100g",
		"vitamin_b12_mcg_per_100g", "vitamin_c_mg_per_100g", "magnesium_mg_per_100g",
		"potassium_mg_per_100g", "zinc_mg_per_100g", "nutriment_computed_at",
		"last_logged_quantity_g",
	} {
		var exists bool
		err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'products' AND column_name = $1)`,
			col,
		).Scan(&exists)
		require.NoError(t, err, "lookup products.%s", col)
		require.True(t, exists, "products.%s should exist after Up", col)
	}

	var recipeAllowed bool
	err = pool.QueryRow(context.Background(),
		`SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = 'products_source_check'
			  AND pg_get_constraintdef(oid) ILIKE '%recipe%'
		)`,
	).Scan(&recipeAllowed)
	require.NoError(t, err)
	require.True(t, recipeAllowed, "products_source_check should accept 'recipe' after Up")
}

// TestMigrate_DirtyStateDetectionAndRecovery simulates a half-applied migration
// (the golang-migrate `dirty` flag set) and verifies that Migrate surfaces an
// actionable error naming the version + `--force` recovery command, that Force
// clears the flag, and that a subsequent Migrate then succeeds.
func TestMigrate_DirtyStateDetectionAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed dirty-state test in short mode")
	}
	ctx := context.Background()
	dsn := startPostgres(t)
	require.NoError(t, Migrate(dsn), "initial Up")

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var head int
	require.NoError(t, pool.QueryRow(ctx, `SELECT version FROM schema_migrations`).Scan(&head))

	// Simulate a migration that failed partway: the version row is marked dirty.
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty = true`)
	require.NoError(t, err)

	// Detection: Migrate must fail with an actionable, named message.
	err = Migrate(dsn)
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "DIRTY")
	require.Contains(t, msg, strconv.Itoa(head), "error should name the dirty version")
	require.Contains(t, msg, "--force", "error should name the recovery command")

	// Recovery: Force clears the dirty flag at the head version.
	require.NoError(t, Force(dsn, head))
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT dirty FROM schema_migrations`).Scan(&dirty))
	require.False(t, dirty, "Force should clear the dirty flag")

	// A subsequent Migrate now succeeds (no pending migrations to apply).
	require.NoError(t, Migrate(dsn), "re-migrate after force should succeed")
}
