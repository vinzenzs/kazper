## Context

Kazper is a single-user Go + Gin + Postgres backend; one Cobra binary with `serve` / `mcp` / `migrate` subcommands. The database (migration head `054`) holds 42 tables of personal nutrition/training history. The in-flight sibling `add-data-backup` adds `pg_dump -Fc` physical backup/restore and a Helm CronJob — full-fidelity but opaque, Postgres-major-version-coupled, and unreadable by humans. Its design explicitly lists JSON export as a non-goal ("JSON export is portability, not backup; nothing consumes it today"). This change is the complementary **logical** layer: a portable JSON export that survives Postgres version changes, supports inspection/diffing, enables migration to a fresh instance, and answers "give me my data". It must not become a second backup cron — scheduling, retention, and restore drills stay with `data-backup`.

## Goals / Non-Goals

**Goals:**
- One command produces a single JSON document containing *all* user data, with a manifest (format version, timestamp, app version, migration head, row counts).
- One command restores that document into an empty database with FK integrity intact.
- Round-trip fidelity: export → import into a fresh DB → export again is byte-identical modulo the manifest timestamp.
- A structural guarantee that future migrations cannot silently drop tables out of the export (drift guard).

**Non-Goals:**
- No scheduled/retained runs, no Helm objects, no restore drill — that is `data-backup`'s contract.
- No merge/upsert or partial/selective import (single user; merge semantics — conflicting UUIDs, singleton rows, linked FK subgraphs — are a swamp with no consumer).
- No REST export endpoint in v1 (see Decision 1; a read-only `GET /export` is a possible follow-up, not needed now).
- No schema in the export — the export carries data only; schema comes from `kazper migrate` (the migrations are embedded in the binary).
- No secrets export — `garmin_tokens` is excluded unconditionally (see Decision 4).

## Decisions

1. **Surface: CLI subcommands `kazper export` / `kazper import`, talking to the database directly.**
   Precedent: `kazper migrate` already runs standalone against `DATABASE_URL` with the shared config loader and the CLI spec's exit-code contract (0 on success, non-zero + logged error on failure). Direct-DB CLI beats REST endpoints here: no HTTP timeout/streaming concerns for a multi-megabyte body, no auth-token ceremony for a local operator task, and — critically — import must run against an *empty* database, i.e. exactly when the server is not (or should not be) serving. Being pure Go it also works in the distroless production image, where `pg_dump` cannot (the sibling had to use a separate `postgres:16` client image for that reason). A read-only REST `GET /export` (nice for the mobile companion someday) is deferred, not rejected. Alternatives considered: REST endpoints (rejected above), Task targets shelling to `psql`+`jq` (rejected: no drift guard, no typed guards, untestable).

2. **Format: one JSON document — manifest object + per-table row arrays, deterministic, row-per-line.**
   Shape:
   ```json
   {
     "manifest": {
       "format_version": 1,
       "exported_at": "2026-07-13T09:00:00Z",
       "app_version": "v0.42.0",
       "migration_head": 54,
       "tables": { "products": 210, "workouts": 987, "...": 0 }
     },
     "tables": {
       "products": [ {"id": "…", …}, … ],
       "…": []
     }
   }
   ```
   - **Single file over a directory/archive**: "give me my data" should be one artifact; the manifest and the data cannot drift apart; trivially piped, copied, attached. A tar of per-table files was rejected — it adds an archive dependency for zero benefit at this scale (single user; the whole DB is megabytes).
   - **Deterministic**: tables serialized in fixed inventory order; rows ordered by primary key; per-row keys in stable column order. Two exports of the same data differ only in `exported_at` — this is what makes the export diffable and the round-trip test meaningful.
   - **Pretty vs compact**: hybrid — structural indentation for the manifest and table keys, but each **row is one compact JSON object on its own line**. That yields line-per-row diffs (`git diff`/`diff` show exactly which rows changed) without the size bloat of fully-indented rows, and lets both writer and reader stream row-by-row without holding a table in memory. Full NDJSON as the file format was rejected: it loses the single self-describing document (manifest + named tables) and buys nothing the row-per-line convention doesn't already give.
   - **Canonical values via Postgres, not per-table Go structs**: rows are exported with `row_to_json(t)` (ordered by PK) and imported with `json_populate_record`-style inserts. Postgres canonicalizes numerics, timestamps, and arrays; the Go code stays generic (inventory-driven, no 42 hand-maintained structs), which is also what makes the drift guard cheap to keep honest. UUIDs are preserved verbatim, which is what keeps every FK (`meal_entries.workout_id` → `workouts`, `plan_slots` → templates, `race_legs` → `races`, `product_components` → `products`, …) valid after import.
   - **Streams interaction**: if `persist-activity-streams` lands, its `workout_streams` table (`REAL[]` sample arrays) will dominate export size. The drift guard *forces* that change to classify the table when its migration lands; the recorded recommendation is **include it** (round-trip fidelity is the point; streams are only re-fetchable within Garmin's backfill window). No size-management flag in v1 — revisit if exports actually get unwieldy (open question).

3. **Table inventory: explicit, exhaustive, with a fail-loud drift guard.**
   `internal/dataexport` carries a hardcoded inventory classifying every table at migration head `054`:
   - **Exported (37)** — user data: `products`, `product_components`, `meal_entries`, `planned_meals`, `shopping_items`, `hydration_entries`, `workouts`, `workout_sets`, `workout_splits`, `workout_best_efforts`, `workout_fuel_entries`, `body_weight_entries`, `nutrition_goals`, `daily_goal_overrides`, `goal_templates`, `training_phases`, `workout_templates`, `multisport_templates`, `training_plans`, `plan_weeks`, `plan_slots`, `macrocycles`, `races`, `race_legs`, `recovery_metrics`, `fitness_metrics`, `hydration_balance_metrics`, `daily_summary`, `health_vitals`, `gear`, `personal_records`, `achievements`, `devices`, `athlete_config`, `chat_sessions`, `chat_messages`, `coach_memory`. Chat history and coach memory are user history — included. The Garmin-mirrored metric tables (`recovery_metrics`, `daily_summary`, …) are included even though partially re-fetchable: backfill windows are finite and inclusion is cheap.
   - **Excluded (5)** — transient, instance-bound, or secret: `idempotency_records` (replay cache; stale and meaningless on a new instance), `sync_runs` (bridge-invocation ops log, recreated on next sync), `push_tokens` (FCM device registrations; the companion re-registers), `relogin_latch` (single seeded guard row), `garmin_tokens` (secret — Decision 4).
   - `schema_migrations` is not data; its version is captured as `manifest.migration_head`.
   - **Drift guard**: at export time, enumerate live base tables in `public` (information_schema, minus `schema_migrations`); if any table is on neither list — or an inventoried table is missing — the export **fails with a non-zero exit naming the table**. A future migration therefore cannot ship without an explicit include/exclude decision. Alternative — "export whatever exists" — rejected: it silently exports future secret tables and silently changes format contents.

4. **`garmin_tokens` is always excluded; no `--include-secrets` flag.**
   The stored blob is AES-256-GCM ciphertext keyed by `GARMIN_TOKEN_ENC_KEY` (config-resident, never in the DB — per the `garmin-auth` spec). Exporting the ciphertext is useless for portability without also exporting the key, and shipping key + ciphertext in a plaintext JSON file is a secret-with-extra-steps. A re-login via the existing MCP login flow is cheap and is the documented post-import step for a new instance. An `--include-secrets` flag was considered and rejected for v1: it complicates the format contract for a secret that is trivially re-creatable.

5. **Import v1: empty database only, no `--force` wipe.**
   Import refuses — non-zero exit, error listing the offending tables — if any *exported-inventory* table has rows (excluded tables like the migration-seeded `relogin_latch` row don't count). No merge/upsert (see Non-Goals). No destructive `--force` wipe-and-import either: import's safety contract is "cannot destroy data, ever", and wiping is already a separate, deliberate operation (`task db:wipe` locally; documented drop/recreate in production per `BACKUP.md`). The two-step — wipe, then import — keeps the destructive intent typed explicitly. Revisit `--force` only if the two-step proves annoying in practice (open question).

6. **Import runs in one transaction, inserting in FK-topological order; no deferred constraints.**
   The FK graph is read from `pg_catalog` at import time and topologically sorted (`workouts` before `meal_entries`/`hydration_entries`/`workout_fuel_entries`/`workout_sets`/…, `products` before `product_components`, `races` before `race_legs`, templates before `plan_slots`, `macrocycles` before dependents, …) — self-maintaining as the schema grows, no hardcoded order to rot. `SET CONSTRAINTS ALL DEFERRED` was rejected: the existing FKs are not declared `DEFERRABLE` and changing that would require a migration for no gain. After all inserts, per-table row counts are verified against the manifest inside the same transaction; any mismatch or FK violation rolls back the entire import — no partial state, matching the idempotency-era "no torn writes" posture.

7. **Compatibility gates: format version and exact migration-head match.**
   - `format_version` in the manifest starts at `1`; import refuses files with a version greater than it supports (older versions are handled per-version if the format ever evolves).
   - Import requires the target database's migration head to **equal** `manifest.migration_head` exactly. Rows are serialized against a specific schema shape; mapping old rows onto a newer schema is the migrations' job, not the importer's. The refusal error is instructive: if the target DB is behind the manifest, run `kazper migrate` with the matching binary; if the manifest is older than the current binary's head, import with the binary version recorded in `manifest.app_version`, then upgrade and `kazper migrate`. Auto-running migrations inside import was rejected — import should never mutate schema, and `migrate` only knows "latest", which may overshoot an older manifest.

8. **Round-trip fidelity is a spec requirement and the core test.**
   Export from a populated DB → `kazper import` into a freshly migrated empty testcontainers DB → export again → assert byte-identical output after normalizing `manifest.exported_at`. This single test transitively proves UUID preservation, FK integrity, canonical serialization, deterministic ordering, and count verification.

9. **MCP: no new tools.**
   The MCP-mirrors-REST-1:1 convention applies to REST endpoints; this change adds none. CLI subcommands are explicitly outside the MCP surface — stated here so the absence of a tool is a decision, not an omission.

## Risks / Trade-offs

- [Schema drift: a future migration adds a table the exporter doesn't know] → the explicit-inventory drift guard fails the export loudly, naming the table; a dedicated test creates a throwaway table and asserts the failure. This converts silent data loss into a compile-a-decision checkpoint.
- [Secrets in a plaintext JSON file] → `garmin_tokens` unconditionally excluded (Decision 4); export contains no credentials, keys, or token material. `push_tokens`/`idempotency_records` (which embed response bodies) are excluded too.
- [Canonical-form instability across Postgres versions breaks byte-identical diffs] → `row_to_json` output for the types in use (uuid, text, numeric, timestamptz, date, integer, boolean, arrays) is stable in practice; the round-trip test pins behavior per CI Postgres version. Cross-major-version diffs may differ cosmetically — the *import* still works, which is the actual portability guarantee; note in docs.
- [Large exports if `workout_streams` lands] → accepted for v1 (Decision 2); the drift guard forces a conscious classification when that migration ships, and a size-management flag is the named follow-up if needed.
- [Import into a half-provisioned DB (migrations behind)] → the migration-head gate refuses with the exact remedial command; the empty-check runs after the head check so errors arrive in a sensible order.
- [Operator confusion between backup and export] → docs position them side by side: `pg_dump` for disaster recovery (fast, exact, version-coupled), JSON export for portability/inspection/migration (readable, version-independent). Neither replaces the other.

## Migration Plan

None. No schema change, no migration pair, no data backfill, no deploy coupling. Purely additive CLI surface; rollback is "don't use the subcommands".

## Open Questions

- Should a later version add a read-only REST `GET /export` (streamed) so the mobile companion can pull a data copy without shell access? Deferred until a client wants it; it would then get an MCP mirror per convention.
- If `persist-activity-streams` makes exports large, add `--skip-table <name>` (export-side only, recorded in the manifest so import can verify intent)? Deferred until size is a real problem.
- Revisit `--force` wipe-and-import if the wipe-then-import two-step proves annoying in real restore-to-new-instance runs.
