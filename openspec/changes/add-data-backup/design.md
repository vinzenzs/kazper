## Context

Kazper's Postgres database is the single copy of months of personal nutrition/training history. There are two deployment shapes to cover:

- **Local dev**: Postgres runs in a compose-managed container (`task db:up`), data in a named volume. `task db:wipe` is one typo away from deleting it. `DATABASE_URL` lives in `.env.local` (dev default well-known).
- **Production**: single-replica Deployment via the Helm chart; Postgres is **externally provisioned** (the chart deliberately ships no DB) and reachable only through the `DATABASE_URL` secret. The app image is distroless — no shell, no pg client tools.

The 2026-07-13 gap analysis flagged the absence of any backup/export path as the top operational risk. The `single-user-by-design` decision means we do not need per-user export, GDPR portability shapes, or an API surface — an operator-grade dump/restore story is the whole requirement.

## Goals / Non-Goals

**Goals:**
- One-command full-fidelity local backup and restore (`task db:backup` / `task db:restore`).
- An opt-in, scheduled, retained in-cluster dump for the production database.
- A written, tested restore drill — the deliverable is *restorability*, not dump files.

**Non-Goals:**
- No API export endpoint (JSON export is portability, not backup; nothing consumes it today).
- No off-site/object-storage upload (PVC first; S3/rclone is a follow-up if the cluster's storage isn't trusted).
- No point-in-time recovery / WAL archiving — daily-granularity dumps are proportionate to a single-user system.
- No backup of the Garmin bridge's session state or `.env.local` secrets (recoverable by re-login / re-provisioning).

## Decisions

1. **`pg_dump -Fc` (custom format) over plain SQL or JSON.**
   Custom format is compressed, supports selective/parallel restore via `pg_restore`, and captures schema + data + the `schema_migrations` version table in one artifact, so a restored database is exactly what `MIGRATE_ON_START` expects (migrate no-ops). A JSON export would need bespoke code per table and can't rebuild constraints/indexes. Alternative considered: `pg_dumpall` — rejected, we want one database, not cluster-wide roles.

2. **Run pg tools inside the Postgres container locally (`compose exec`), and in a `postgres:16` client image in-cluster.**
   The host may not have Postgres client tools; the app image (distroless) definitely doesn't. The Postgres server container already carries a version-matched `pg_dump`. In-cluster, the CronJob pod uses the official `postgres:16` image purely for its client binaries — version-matched to the server major to avoid dump-format skew. Alternative: install `pg_dump` on the host / bake tools into the app image — rejected (host drift; bloats and un-distroles the app image).

3. **Helm backup is a `CronJob` + PVC guarded by `backup.enabled=false`.**
   Default-off keeps the chart's "fresh install yields exactly these objects" contract intact — no deployment-pipeline spec delta. The CronJob mounts a chart-managed PVC at `/backups`, runs `pg_dump -Fc` to a timestamped file, then prunes to the newest `backup.retention` files (`ls -1t | tail … rm` in the job script). `DATABASE_URL` comes from the same Secret reference the Deployment uses, so `existingSecret` installs work unchanged. Alternative: a sidecar or in-app scheduler — rejected; backup should not share the app's lifecycle or its failure modes.

4. **Retention is keep-last-N files, not time-based.**
   Trivially auditable (`ls`), no clock-edge cases, bounded disk. Default `retention: 14` with `schedule: "0 3 * * *"` ≈ two weeks of dailies.

5. **`task db:restore` targets the *local* database only and demands confirmation.**
   Restore drops and recreates the `nutrition` database inside the local container (`pg_restore --clean --create` against the `postgres` maintenance DB). It refuses to run without `FILE=` and prompts before dropping (skippable with `FORCE=1` for scripted drills). Production restore is deliberately manual and documented in `BACKUP.md` rather than automated — a one-shot destructive op on prod should be typed by a human, not hidden behind a task target.

6. **Restore drill verifies via the app, not just `pg_restore` exit code.**
   `BACKUP.md`'s drill ends with booting the binary against the restored DB and checking `/readyz` + a known read (e.g. `GET /api/v1/goals`) — proving migrations align and data is readable, which is the actual definition of "the backup works".

## Risks / Trade-offs

- [Dump runs against the live DB] → `pg_dump` takes a consistent snapshot via MVCC; no locking concern at this write volume. No mitigation needed beyond scheduling at 03:00.
- [PVC lives in the same cluster/storage as the database] → acknowledged residual risk; the capability's value is still real (protects against app-level mistakes, bad migrations, accidental deletes — the likeliest loss modes). Off-site upload is the named follow-up.
- [`postgres:16` client vs server version skew] → pin `backup.image` in values; document that it should track the server major. `pg_dump` newer-than-server is supported; older-than-server is not.
- [Retention prune deletes the only good backup after N bad ones] → prune runs only after a successful dump (`set -e` before the prune step), so failures never rotate out existing files.
- [Local `task db:backup` while the container is down] → target `deps: [db:up]` so the container is started first, matching existing task ergonomics.

## Migration Plan

Purely additive: merge, then optionally `helm upgrade --set backup.enabled=true --set backup.storage.size=5Gi`. Rollback = disable the flag (PVC survives unless manually deleted). No app deploy required; no data migration.

## Open Questions

- Should the CronJob emit a freshness signal (e.g. a `kazper_backup_last_success` timestamp somewhere observable)? Deferred — the gap analysis separately flagged the absence of metrics; when a metrics story lands, backup freshness should be one of the first gauges.
