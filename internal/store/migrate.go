package store

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations
var migrationsFS embed.FS

// newMigrator builds a *migrate.Migrate wired to the embedded migration files
// and the supplied database URL. Exposed for tests that need direct access to
// Up/Down/Steps; production code goes through Migrate.
func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}
	return m, nil
}

// Migrate runs all up migrations embedded in the binary against databaseURL.
// If a prior run left the schema in a dirty state (a migration that failed
// partway), the returned error names the dirty version and the exact
// `kazper migrate --force <version>` recovery command instead of only the raw
// driver error — so a MIGRATE_ON_START boot loop is diagnosable from the logs.
func Migrate(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return annotateDirty(m, fmt.Errorf("apply migrations: %w", err))
	}
	return nil
}

// Force clears a dirty migration state by pinning the recorded schema version
// to the given value WITHOUT running any migration (wraps golang-migrate's
// Force). The operator sets it to the last successfully-applied version, then
// re-runs Migrate to apply the rest.
func Force(databaseURL string, version int) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Force(version); err != nil {
		return fmt.Errorf("force migration version %d: %w", version, err)
	}
	return nil
}

// annotateDirty wraps err with actionable recovery guidance when the migration
// state is dirty; otherwise it returns err unchanged.
func annotateDirty(m *migrate.Migrate, err error) error {
	version, dirty, verr := m.Version()
	if verr != nil || !dirty {
		return err
	}
	return fmt.Errorf("%w\nmigration state is DIRTY at version %d — a migration failed partway. "+
		"Inspect that migration (and its .down.sql) first, then clear the flag with:\n"+
		"    kazper migrate --force <version>\n"+
		"where <version> is the last successfully-applied migration (often %d); then re-run `kazper migrate`",
		err, version, version-1)
}
