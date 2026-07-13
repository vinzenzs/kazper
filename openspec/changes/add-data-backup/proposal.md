## Why

The Postgres database holds months of irreplaceable single-user history — meals, workouts, weight, coach memory, training plans — and the system has **no backup or export path at all** (the only "export" today is the Garmin FIT proxy, which re-fetches Garmin's copy of Garmin's data). A disk failure, a bad `task db:wipe`, or a botched migration on the production database is unrecoverable data loss. This was the highest irreplaceability-to-cost item in the 2026-07-13 gap analysis.

## What Changes

- **Local backup/restore Task targets**: `task db:backup` runs `pg_dump -Fc` (custom format, includes schema + data + the golang-migrate version table) against the configured `DATABASE_URL`, writing a timestamped dump to a git-ignored `backups/` directory. `task db:restore FILE=<dump>` restores a dump into the local dev database (drop-and-recreate semantics, explicit confirmation).
- **Opt-in scheduled backup in the Helm chart**: a `backup.enabled` flag (default **false**) renders a Kubernetes `CronJob` that runs `pg_dump -Fc` against the chart's `DATABASE_URL` secret on a configurable schedule, writing to a chart-managed PVC with simple keep-last-N retention. Disabled, the chart renders exactly the same objects as today.
- **A documented restore drill**: `BACKUP.md` at the repo root walks through taking a backup, restoring it into a scratch database, and verifying the app boots against it (`kazper migrate` no-ops, `/readyz` green) — because an untested backup is not a backup.
- No Go code changes, no migration, no new API route, no MCP tool. This is a pure operational capability.

## Capabilities

### New Capabilities
- `data-backup`: the backup-and-restore contract for the Postgres-resident personal history — local dump/restore Task targets, the opt-in in-cluster scheduled dump with retention, and the documented restore drill.

### Modified Capabilities
<!-- none — the deployment-pipeline default-install object set is unchanged (backup.enabled defaults false), and local-dev-tooling's existing requirements are untouched (new targets are additive and spec'd under data-backup) -->

## Impact

- **Taskfile.yml**: new `db:backup` / `db:restore` targets alongside the existing `db:up`/`db:down`/`db:wipe`; they reuse the same compose-runtime detection and read `DATABASE_URL` from `.env.local` (falling back to the dev default). `pg_dump`/`pg_restore` run **inside the Postgres container** (`compose exec`) so no host Postgres client tooling is required.
- **deploy/helm/kazper/**: new `templates/backup-cronjob.yaml` + `templates/backup-pvc.yaml` guarded by `backup.enabled`; `values.yaml` gains a `backup:` block (`enabled`, `schedule`, `image`, `retention`, `storage`). The CronJob uses a `postgres:16` client image (the distroless app image has no shell or pg tools) and sources `DATABASE_URL` from the same Secret as the Deployment (works with `existingSecret`).
- **Docs**: new `BACKUP.md`; `README.md` and `RUN_LOCAL.md` get a short pointer.
- **`.gitignore`**: `backups/`.
- **Not affected**: Go source, migrations, REST/MCP surface, `docs/` (no `task swag` needed).
