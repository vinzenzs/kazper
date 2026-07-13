## ADDED Requirements

### Requirement: migrate detects and recovers a dirty migration state

The `migrate` subcommand SHALL accept a `--force <version>` flag wrapping golang-migrate's `Force` to clear a dirty migration state; a bare `--force` without a version SHALL be rejected. When any migration run fails — via the subcommand or via `MIGRATE_ON_START` at serve boot — and the migration state is dirty, the logged error SHALL name the dirty version and the exact recovery command (`kazper migrate --force <version>`) along with guidance to inspect the failed migration first, instead of only surfacing the raw driver error.

#### Scenario: Dirty state produces an actionable error

- **WHEN** a migration fails partway and the process is started again with `MIGRATE_ON_START=true`
- **THEN** startup fails with a message naming the dirty version and the `kazper migrate --force <version>` recovery command

#### Scenario: Force clears the dirty flag

- **WHEN** the operator runs `kazper migrate --force <version>` against a database with a dirty migration state
- **THEN** the dirty flag is cleared at that version, the process exits 0
- **AND** a subsequent `kazper migrate` applies pending migrations normally

#### Scenario: Bare force is rejected

- **WHEN** the operator runs `kazper migrate --force` without a version
- **THEN** the command exits non-zero without modifying migration state
