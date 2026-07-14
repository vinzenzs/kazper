# Tasks — add-threshold-history

## 1. Migration

- [x] 1.1 Verify the current migration head before scaffolding: `ls internal/store/migrations | tail` — head is `054_sync_run_summary_partial` at proposal time, but `add-per-sport-tss` and `add-race-pacing-plan` also claim slots and out-of-band work has taken numbers before. Then `task migrate:new NAME=add_athlete_config_history`.
- [x] 1.2 Up migration: create `athlete_config_history` — `effective_from DATE PRIMARY KEY`, the 16 physiology columns mirroring `athlete_config` (same types and `> 0` CHECKs), `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
- [x] 1.3 Seed in the same up migration: `INSERT INTO athlete_config_history (effective_from, <physiology cols>) SELECT DATE '1970-01-01', <physiology cols> FROM athlete_config` (0 or 1 rows; pure SQL). Down migration: `DROP TABLE athlete_config_history`.

## 2. Repo + types (`internal/athleteconfig/`)

- [x] 2.1 Add a `ThresholdSnapshot` type in `types.go`: `EffectiveFrom` (date) + the 16 physiology fields (reuse/embed the `AthleteConfig` field set) + `created_at`/`updated_at`, JSON tags with `omitempty` on nullables.
- [x] 2.2 Add history repo methods on `store.Querier` (tx-compatible): `UpsertSnapshot(effectiveFrom, state)`, `DeleteSnapshot(effectiveFrom)`, `LatestBefore(date)` (strictly-before, for dedup), `ListHistory(from, to *date)` ascending, `AsOf(date)` (`effective_from <= date ORDER BY effective_from DESC LIMIT 1`, nil-row → `(nil, nil)`).

## 3. Service: history write hook + as-of lookup

- [x] 3.1 Add a pointer-aware `physiologyEqual(a, b *AthleteConfig) bool` over all 16 fields (timestamps excluded), with a unit test covering nil-vs-value and float fields.
- [x] 3.2 Extend `Service.Put` to run singleton upsert + history maintenance in one `pgx.Tx`: load prior state; upsert singleton; if new state differs from prior, upsert today's snapshot — except when the new state equals `LatestBefore(today)`, in which case delete today's snapshot (same-day revert collapse). No-change PUT touches history not at all. `GET`/`PUT /athlete-config` request/response behavior stays byte-identical.
- [x] 3.3 Add `Service.History(ctx, from, to)` and `Service.ConfigAsOf(ctx, date)` (returns `*ThresholdSnapshot`, `(nil, nil)` on empty history). `ConfigAsOf` is provided but deliberately not wired into any existing consumer (IF derivation, zone resolution, TSS/pacing siblings keep current-value reads).

## 4. Handler: GET /athlete-config/history

- [x] 4.1 Add the `GET /athlete-config/history` handler in `handlers.go`, registered in the existing `Register` (same router group; no `httpserver/server.go` change). Parse optional `from`/`to`: malformed → `400 {"error":"date_invalid","field":"from"|"to"}`; `from > to` → `400 {"error":"range_invalid"}`. Respond `{"history":[...]}` ascending, `numfmt.Round1` on the two pace floats at the boundary, `200 {"history":[]}` when empty. Full swag annotations (note the `1970-01-01` seed-sentinel semantics in the description).

## 5. MCP tool

- [x] 5.1 Register `athlete_config_history_get` (TierRead, optional `from`/`to` args, one `GET /athlete-config/history` HTTPCall, body forwarded verbatim, no idempotency key) beside the existing `athlete_config_get` in the `agenttools` registry.
- [x] 5.2 Regenerate the announced-schema golden: `go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/` — the announced surface is registry-derived post `unify-mcp-tool-registry`; do NOT hand-edit an expected-tools list (CLAUDE.md's "bump the list" note predates the registry).

## 6. Tests

- [x] 6.1 Migration/seed: with a pre-existing `athlete_config` row, the migrated DB has exactly one history row at `effective_from = 1970-01-01` mirroring it; on a fresh DB the table is empty and the first PUT creates the first snapshot.
- [x] 6.2 Snapshot-on-change: a PUT changing `ftp_watts` appends today's snapshot with the full new state; the PUT response itself is unchanged from the pre-history behavior.
- [x] 6.3 No-op PUT dedup: re-PUTting an identical body (Garmin daily-sync shape) appends nothing; history row count is stable across repeated identical PUTs.
- [x] 6.4 Same-day semantics: a second change today replaces today's row (still one row for today); a same-day revert to the prior state deletes today's row (no two consecutive rows physiology-identical).
- [x] 6.5 History range read: ascending order; inclusive `from`/`to` filtering; `400 date_invalid` with `field` hint on malformed params; `400 range_invalid` on inverted range; `200 {"history":[]}` when empty; pace floats rounded to 1dp in the response while storage keeps full precision.
- [x] 6.6 As-of resolution: date before the first snapshot after seeding resolves the seed; a date on/after a later snapshot's `effective_from` resolves that snapshot; empty history resolves `(nil, nil)`.
- [x] 6.7 Regression: existing athlete-config handler tests stay green (GET/PUT shapes, `athlete_config_value_invalid`, `idempotency_unsupported_for_put`), and IF derivation still reads the singleton's current `ftp_watts`.

## 7. Docs & verification

- [x] 7.1 Run `task swag` to regenerate `docs/` for the new endpoint (required after any handler change).
- [x] 7.2 Add the `athlete_config_history_get` row to the README MCP tool table (match how sibling read tools are documented).
- [x] 7.3 Run `go test -count=1 ./internal/athleteconfig/...`, the MCP integration + golden tests, `task vet`, then `task test` (re-run isolated with `-p 1` on testcontainers boot contention).
