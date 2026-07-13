## 1. Table inventory + dataexport package

- [x] 1.1 Create `internal/dataexport/` with the explicit table inventory: export list (37 tables per design Decision 3) and exclusion list (`idempotency_records`, `sync_runs`, `garmin_tokens`, `push_tokens`, `relogin_latch`); verify the lists against the live migrations head first (`ls internal/store/migrations/ | tail` — head is around `054`; out-of-band work such as `persist-activity-streams` may have taken a slot, in which case classify the new table explicitly before proceeding).
- [x] 1.2 Implement the drift guard: enumerate live base tables in `public` (information_schema, excluding `schema_migrations`) and return a named error if any table is unclassified or an inventoried table is missing.
- [x] 1.3 Implement the export writer: manifest (format_version=1, exported_at RFC 3339 UTC, app_version from build metadata, migration_head from the migrate version table, per-table counts) + per-table rows via `row_to_json` ordered by primary key, tables in fixed inventory order, one compact row object per line.
- [x] 1.4 Implement the import loader: manifest validation (format version, migration-head equality), empty-database check over exported-inventory tables only, single transaction, insert order topologically sorted over the live FK graph from `pg_catalog`, post-insert count verification against the manifest, full rollback on any error.

## 2. CLI subcommands

- [x] 2.1 Add `cmd/kazper/export.go`: Cobra `export` subcommand (pattern-match `migrate.go` — shared config loader, DB-only validation), stdout by default, `--out <path>` flag, logs to stderr, exit codes per the cli spec delta.
- [x] 2.2 Add `cmd/kazper/import.go`: Cobra `import` subcommand with required positional file argument (`-` = stdin), guard errors naming the specific refusal and remedial step, success summary of tables/rows imported.
- [x] 2.3 Register both subcommands on the root command and confirm `kazper --help` lists them with one-line descriptions.

## 3. Tests (testcontainers Postgres)

- [x] 3.1 Round-trip integration test: seed a populated DB across FK-linked tables (workouts + linked meals/hydration/workout-fuel, products + components, races + legs, plan tables, chat sessions + messages, coach_memory), export, import into a second freshly migrated container DB, export again, assert byte-identical after normalizing `manifest.exported_at`.
- [x] 3.2 Drift-guard test: `CREATE TABLE` a throwaway unclassified table, assert export exits with an error naming it and writes no document.
- [x] 3.3 Guard tests: non-empty target refused (listing tables; seeded `relogin_latch` alone does not block), newer `format_version` refused, migration-head mismatch refused with remedy text, malformed file refused, mid-import failure leaves zero imported rows.
- [x] 3.4 Secret-exclusion test: store a garmin token blob + rows in each excluded table, export, assert none of the five excluded tables (nor any ciphertext/nonce material) appear in the document.
- [x] 3.5 Determinism test: two consecutive exports of an unchanged DB are byte-identical after `exported_at` normalization.

## 4. Docs + verification

- [x] 4.1 Add an export/import section to `README.md` and `RUN_LOCAL.md`: usage, positioning vs `pg_dump` backup (physical/DR vs logical/portable — cross-reference `BACKUP.md` if `add-data-backup` has landed), and the post-import notes (Garmin re-login required; companion re-registers push token). No `task swag` needed — this change adds no REST surface or handler annotations.
- [x] 4.2 Run `task vet` and `task test` (dataexport package solo via `go test -count=1 ./internal/dataexport/... -p 1` if testcontainers contention bites); all green.
