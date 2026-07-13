## 1. Local backup/restore Task targets

- [x] 1.1 Add `backups/` to `.gitignore`
- [x] 1.2 Add `task db:backup` to `Taskfile.yml`: `deps: [db:up]`, reuse the existing compose-runtime detection, run `pg_dump -Fc` via `compose exec` on the Postgres service against the configured `DATABASE_URL` (`.env.local` value, dev default fallback), write `backups/kazper-<UTC timestamp>.dump`
- [x] 1.3 Add `task db:restore` to `Taskfile.yml`: require `FILE=` (exit non-zero with a clear message when missing), confirmation prompt before dropping (skippable with `FORCE=1`), `pg_restore --clean --create` via `compose exec` against the `postgres` maintenance database, local container only
- [x] 1.4 Verify round-trip manually: seed data → `task db:backup` → `task db:wipe` → `task db:up` → `task db:restore FILE=… FORCE=1` → `kazper migrate` no-ops → known API read returns the data

## 2. Helm opt-in scheduled backup

- [x] 2.1 Add `backup:` block to `deploy/helm/kazper/values.yaml` (`enabled: false`, `schedule: "0 3 * * *"`, `image: postgres:16`, `retention: 14`, `storage.size`) with comments
- [x] 2.2 Add `templates/backup-pvc.yaml` guarded by `backup.enabled`
- [x] 2.3 Add `templates/backup-cronjob.yaml` guarded by `backup.enabled`: `pg_dump -Fc` to timestamped file on the PVC mount, `set -e`, prune to newest `backup.retention` files only after a successful dump; `DATABASE_URL` from the same Secret reference as the Deployment (honoring `existingSecret`)
- [x] 2.4 Verify with `helm template`: default install renders no backup objects (object set unchanged); `backup.enabled=true` renders CronJob + PVC; `existingSecret` path wires the CronJob env to the external secret

## 3. Restore drill documentation

- [x] 3.1 Write `BACKUP.md`: local backup/restore, retrieving a dump from the cluster PVC (`kubectl cp` / ephemeral pod), manual production-restore procedure, and the app-level verification steps (`migrate` no-op, `/readyz`, known read)
- [x] 3.2 Add pointers to `BACKUP.md` in `README.md` and `RUN_LOCAL.md`
- [x] 3.3 Run the drill once end-to-end locally following only `BACKUP.md` as written; fix any step that didn't match reality

## 4. Wrap-up

- [x] 4.1 Confirm no Go/API surface was touched (no `task swag` needed); run `task vet` + affected checks anyway for hygiene
- [x] 4.2 Update `openspec/changes/add-data-backup/` task states and propose the `feat(backup): …` commit
