## ADDED Requirements

### Requirement: Local one-command backup via Task

The system SHALL provide a `task db:backup` target that produces a full-fidelity backup of the configured database using `pg_dump` in custom format (`-Fc`), executed inside the compose-managed Postgres container so no host Postgres client tooling is required. The dump SHALL include schema, data, and the golang-migrate version table, and SHALL be written to a git-ignored `backups/` directory with a timestamped filename.

#### Scenario: Backup produces a timestamped custom-format dump

- **WHEN** the user runs `task db:backup` with the local database running and containing data
- **THEN** a file matching `backups/kazper-<UTC timestamp>.dump` is created
- **AND** the file is a valid `pg_dump` custom-format archive (`pg_restore --list` succeeds against it)

#### Scenario: Backup starts the database if needed

- **WHEN** the user runs `task db:backup` while the Postgres container is not running
- **THEN** the container is started first (same behavior as targets that depend on `db:up`) and the backup proceeds

#### Scenario: Backup artifacts are never committed

- **WHEN** a dump file exists under `backups/`
- **THEN** `git status` does not list it (the directory is git-ignored)

### Requirement: Local restore via Task with explicit confirmation

The system SHALL provide a `task db:restore FILE=<dump>` target that restores a custom-format dump into the local compose-managed database using `pg_restore` with drop-and-recreate semantics. The target SHALL refuse to run without a `FILE` argument, SHALL prompt for confirmation before dropping the existing local database (bypassable with `FORCE=1` for scripted drills), and SHALL NOT offer any code path that targets a non-local database.

#### Scenario: Restore round-trips the data

- **WHEN** the user takes a backup, wipes the local database, and runs `task db:restore FILE=<that dump>` confirming the prompt
- **THEN** the restore completes successfully
- **AND** a subsequent `kazper migrate` against the restored database applies no new migrations
- **AND** previously logged data is readable through the API

#### Scenario: Missing FILE argument is rejected

- **WHEN** the user runs `task db:restore` with no `FILE` argument
- **THEN** the target exits non-zero with a message naming the required argument and no database change occurs

#### Scenario: Declining the confirmation aborts

- **WHEN** the user runs `task db:restore FILE=<dump>` and declines the confirmation prompt
- **THEN** the target exits without modifying the database

### Requirement: Opt-in scheduled backup in the Helm chart

The Helm chart SHALL support an opt-in scheduled backup via a `backup.enabled` value that defaults to **false**. When enabled, the chart SHALL render a Kubernetes `CronJob` and a chart-managed `PersistentVolumeClaim`; the CronJob SHALL run `pg_dump -Fc` against the same `DATABASE_URL` Secret reference the Deployment uses (including the `existingSecret` path), using a version-pinned Postgres client image (`backup.image`), writing a timestamped dump to the PVC. The schedule (`backup.schedule`), retention count (`backup.retention`), and PVC size (`backup.storage.size`) SHALL be configurable values.

#### Scenario: Disabled by default renders no backup objects

- **WHEN** the chart is installed without setting `backup.enabled`
- **THEN** no `CronJob` and no backup `PersistentVolumeClaim` are rendered
- **AND** the rendered object set is identical to the chart's pre-backup contract

#### Scenario: Enabled renders CronJob and PVC wired to the database secret

- **WHEN** the chart is templated with `backup.enabled=true`
- **THEN** a `CronJob` is rendered with the configured `backup.schedule` and the `backup.image`
- **AND** its pod sources `DATABASE_URL` from the same Secret name the Deployment references
- **AND** a `PersistentVolumeClaim` of `backup.storage.size` is rendered and mounted at the CronJob's backup path

#### Scenario: existingSecret installs keep working

- **WHEN** the chart is templated with `backup.enabled=true` and `existingSecret=my-tokens`
- **THEN** the CronJob's `DATABASE_URL` env references `my-tokens` and the chart renders no Secret of its own

### Requirement: Scheduled backups are retained newest-first and pruned only after success

The scheduled backup job SHALL prune the backup directory to the newest `backup.retention` dump files, and SHALL perform pruning only after the current dump has completed successfully, so a failing dump can never rotate out existing good backups.

#### Scenario: Retention keeps the newest N dumps

- **WHEN** the backup job runs with `backup.retention=14` and more than 14 dump files exist after a successful dump
- **THEN** only the 14 newest dump files remain on the PVC

#### Scenario: A failed dump preserves existing backups

- **WHEN** the backup job's `pg_dump` step fails (e.g. the database is unreachable)
- **THEN** the job exits non-zero without deleting any existing dump files

### Requirement: Documented restore drill verified through the application

The system SHALL document the backup and restore procedure in `BACKUP.md` at the repository root, covering: taking a local backup, retrieving a scheduled dump from the cluster PVC, restoring into a scratch database, and the manual production-restore procedure. The drill SHALL define restore success through the application — the binary boots against the restored database, `MIGRATE_ON_START` applies no new migrations, `/readyz` reports ready, and a known read endpoint returns the expected data — not merely through `pg_restore`'s exit code.

#### Scenario: Drill instructions verify via the app

- **WHEN** a reader follows the `BACKUP.md` restore drill end to end against a scratch database
- **THEN** the final steps have them boot the binary against the restored database and observe `/readyz` returning ready and a known read (e.g. `GET /api/v1/goals`) returning the backed-up data

#### Scenario: Entry points reference the drill

- **WHEN** a reader looks for backup guidance in `README.md` or `RUN_LOCAL.md`
- **THEN** each contains a pointer to `BACKUP.md`
