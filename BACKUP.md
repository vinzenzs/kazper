# Backups & restore

Kazper's Postgres database is the **single copy** of months of personal
nutrition and training history — meals, workouts, body weight, coach memory,
training plans. This document is the operator guide for backing it up and, more
importantly, for **restoring** it. An untested backup is not a backup, so the
core of this doc is a restore drill you can run end to end.

The backup artifact is a `pg_dump` **custom-format** archive (`-Fc`): compressed,
and it captures schema + data + the `schema_migrations` version table in one
file. Because the migration state travels with the data, a restored database is
exactly what `MIGRATE_ON_START` expects — `kazper migrate` no-ops against a fresh
restore.

There are two environments:

- **Local dev** — Postgres in a compose-managed container (`task db:up`), driven
  by the `task db:backup` / `task db:restore` targets below.
- **Production** — a single-replica Deployment via the Helm chart against an
  **externally provisioned** Postgres. Backups are an opt-in scheduled `CronJob`
  (`backup.enabled`); restore is a **manual** procedure (see
  [Production restore](#production-restore)).

---

## Local backup

```bash
task db:backup
```

- Starts the Postgres container first if it isn't running (`deps: [db:up]`).
- Runs `pg_dump -Fc` **inside** the container (no host Postgres client tooling
  needed — the server image already carries a version-matched `pg_dump`).
- Reads `DATABASE_URL` from `.env.local` (falling back to the dev default).
- Writes `backups/kazper-<UTC timestamp>.dump`. The `backups/` directory is
  git-ignored — dumps are never committed.

Verify a dump is a valid archive without restoring it:

```bash
docker compose -f compose.yml exec -T postgres pg_restore --list \
  < backups/kazper-<timestamp>.dump | head
```

## Local restore

```bash
task db:restore FILE=backups/kazper-<timestamp>.dump
```

- Requires `FILE=`; exits non-zero with a usage message if it's missing.
- Prompts for confirmation before dropping the local database. Bypass the prompt
  for scripted drills with `FORCE=1`:

  ```bash
  task db:restore FILE=backups/kazper-<timestamp>.dump FORCE=1
  ```

- Runs `pg_restore --clean --create --if-exists` inside the container, connecting
  to the `postgres` maintenance database, so it drops and recreates the target
  database from the dump.
- **Local container only.** There is deliberately no task target that restores a
  non-local database — production restore is typed by a human (below).

> The restore drops and recreates the database, so no other client may hold a
> connection to it. If restore fails with *"database is being accessed by other
> users"*, stop the running app (`Ctrl-C` on `task dev`) and retry.

---

## The restore drill

Run this end to end periodically — the deliverable is *restorability*, not dump
files. Success is defined **through the application**, not just `pg_restore`'s
exit code.

```bash
# 1. Take a backup of your current local database.
task db:backup

# 2. Simulate loss: wipe the container AND its data volume, then bring it back
#    up empty.
task db:wipe
task db:up

# 3. Restore the dump (FORCE=1 skips the confirmation for the drill).
task db:restore FILE=backups/kazper-<timestamp>.dump FORCE=1

# 4. Migrations must NO-OP — the schema_migrations table rode along in the dump,
#    so the restored schema is already at head.
task migrate:up          # expect: "no change"

# 5. Boot the binary against the restored database and check readiness + a known
#    read. `task dev` sources .env.local and serves on :8080.
task dev                 # in another terminal, or background it

# 6. Readiness proves migrations aligned and Postgres is reachable.
curl -fsS http://localhost:8080/readyz && echo "  <- ready"

# 7. A known read proves the DATA came back. Use the mobile token from .env.local.
TOKEN=$(grep ^MOBILE_API_TOKEN .env.local | cut -d= -f2-)
curl -fsS -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/goals | jq .
```

If step 4 reports pending migrations, or step 6/7 fail, the backup is **not**
trustworthy — investigate before relying on it.

---

## Scheduled in-cluster backup (production)

The Helm chart ships an **opt-in** scheduled backup, disabled by default. With
`backup.enabled=false` (the default) the chart renders no `CronJob` and no backup
PVC — a fresh install's object set is unchanged.

Enable it on upgrade:

```bash
helm upgrade kazper deploy/helm/kazper \
  --reuse-values \
  --set backup.enabled=true \
  --set backup.storage.size=5Gi
```

What it does:

- A `CronJob` (`<release>-kazper-backup`) runs on `backup.schedule` (default
  `0 3 * * *`, cluster timezone) using the `backup.image` Postgres client image
  (default `postgres:16` — track your **server** major version; `pg_dump`
  newer-than-server is supported, older is not).
- It runs `pg_dump -Fc` against the **same** `DATABASE_URL` Secret the Deployment
  uses (honoring `existingSecret`), writing `kazper-<timestamp>.dump` to a
  chart-managed PVC (`<release>-kazper-backups`) mounted at `/backups`.
- After a **successful** dump it prunes to the newest `backup.retention` files
  (default `14`). Pruning runs only after the dump succeeds (`set -e`), so a
  failing dump can never rotate out existing good backups.

Trigger an out-of-band run to test it (or to grab a fresh dump before a risky
migration):

```bash
kubectl create job --from=cronjob/<release>-kazper-backup manual-backup-1
kubectl logs job/manual-backup-1
```

### Retrieving a dump from the cluster PVC

The backup PVC is `ReadWriteOnce` and is only mounted while the CronJob runs. To
read dumps off it, mount it from a short-lived pod, then `kubectl cp` the file
out. Save this as `backup-reader.yaml` (replace `<release>`):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: backup-reader
spec:
  restartPolicy: Never
  containers:
    - name: reader
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: backups
          mountPath: /backups
          readOnly: true
  volumes:
    - name: backups
      persistentVolumeClaim:
        claimName: <release>-kazper-backups
```

```bash
kubectl apply -f backup-reader.yaml
kubectl exec backup-reader -- ls -1t /backups           # newest first
kubectl cp backup-reader:/backups/kazper-<timestamp>.dump ./kazper-<timestamp>.dump
kubectl delete pod backup-reader
```

> This works only if the CronJob is not running (RWO can't be mounted twice). If
> the scheduled run overlaps, wait for it to finish or scale the schedule out
> temporarily.

---

## Production restore

Restoring production is a deliberate, manual, one-shot destructive operation — it
is **not** a task target on purpose. The steps:

1. **Stop writers.** Scale the app to zero so nothing holds a connection or runs
   `MIGRATE_ON_START` mid-restore:

   ```bash
   kubectl scale deploy/<release>-kazper --replicas=0
   ```

2. **Get the dump onto a client pod.** Either use the `backup-reader` pod above,
   or run a `postgres:16` pod that has both the dump and `DATABASE_URL`. A quick
   throwaway client with the DB secret wired in:

   ```bash
   kubectl run pg-restore --rm -it --restart=Never --image=postgres:16 \
     --env="DATABASE_URL=$(kubectl get secret <db-secret> -o jsonpath='{.data.DATABASE_URL}' | base64 -d)" \
     -- bash
   # copy the dump into the pod (from another terminal) with:
   #   kubectl cp ./kazper-<timestamp>.dump pg-restore:/tmp/restore.dump
   ```

3. **Restore.** Inside that pod, against the **external** database. `--create`
   needs a maintenance DB to connect to, so point `-d` at `postgres` (swap the
   database in the URL):

   ```bash
   MAINT_URL=$(printf '%s' "$DATABASE_URL" | sed -E 's#(://[^/]+)/[^?]+#\1/postgres#')
   pg_restore --clean --create --if-exists -d "$MAINT_URL" /tmp/restore.dump
   ```

   If your managed Postgres forbids `DROP DATABASE` (many do), restore into the
   existing database instead: `pg_restore --clean --if-exists -d "$DATABASE_URL"
   /tmp/restore.dump`.

4. **Bring the app back and verify — through the application:**

   ```bash
   kubectl scale deploy/<release>-kazper --replicas=1
   kubectl rollout status deploy/<release>-kazper
   ```

   - The pod's own startup runs `MIGRATE_ON_START`; it should apply **no** new
     migrations (they rode along in the dump).
   - `/readyz` on the pod (its readiness probe) must go green.
   - A known read against the API must return the restored data, e.g.
     `GET /api/v1/goals` with the mobile token.

   As locally: readiness + a known read is the definition of "the backup works."

---

## What is *not* backed up

- The Garmin bridge's SSO session state — recover by re-running the one-time MFA
  login.
- `.env.local` / cluster Secrets — recover by re-provisioning.
- No point-in-time recovery / WAL archiving — daily-granularity dumps are the
  chosen granularity for a single-user system.
- Off-site copies — the PVC lives in the same cluster as the database; copying
  dumps off-cluster (object storage) is a follow-up.
