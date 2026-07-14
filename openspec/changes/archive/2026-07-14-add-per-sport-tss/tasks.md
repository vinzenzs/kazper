# Tasks — per-sport computed TSS with provenance

## 1. Migration

- [x] 1.1 Check the current migration head in `internal/store/migrations/` (was `054` at proposal time — out-of-band work may have taken the next slot), then scaffold `task migrate:new NAME=add_workout_tss_source`.
- [x] 1.2 Up migration: `ALTER TABLE workouts ADD COLUMN tss_source TEXT NULL` with CHECK IN `('garmin','manual','power','pace','hr')`; back-fill `UPDATE workouts SET tss_source = CASE WHEN source = 'garmin' THEN 'garmin' ELSE 'manual' END WHERE tss IS NOT NULL`; then add CHECK `(tss IS NULL) = (tss_source IS NULL)`. Down migration drops both constraints and the column.
- [x] 1.3 Migration integration test (testcontainers): column + CHECKs exist; a pre-seeded garmin row with `tss` back-fills to `'garmin'`, a manual one to `'manual'`, and a NULL-tss row stays NULL.

## 2. Types and repo

- [x] 2.1 `internal/workouts/types.go`: add `TSSSource *string \`json:"tss_source,omitempty"\`` to `Workout` (response-only; NOT added to any request-bind struct so caller input is ignored).
- [x] 2.2 `internal/workouts/repo.go`: include `tss_source` in every SELECT projection, INSERT, UPSERT-update, and PATCH column set; keep `tss`/`tss_source` written together everywhere (pairing CHECK).
- [x] 2.3 Add a repo method listing recompute candidates: completed workouts with `tss IS NULL OR tss_source IN ('power','pace','hr')`, plus an update method setting `tss` + `tss_source` (both nullable) by id.

## 3. Service: derivation with precedence

- [x] 3.1 `internal/workouts/service.go`: add `deriveTSS(ctx, w *Workout)` implementing the precedence (explicit > power > pace > hr > none) and formulas from design D1/D2 — duration from the window, `IF > 2.5` skip-guard, `numfmt.Round2` on the stored value, LTHR = `lactate_threshold_hr` else `threshold_hr`; fail-open when `athleteConfigRepo` is nil or thresholds unset.
- [x] 3.2 Wire it into `buildWorkout` after `deriveIntensityFactor` (power TSS consumes the effective IF), gated on `status='completed'`; set `tss_source='garmin'|'manual'` when the caller supplied `tss` (including on planned rows). Covers POST, bulk items, and the upsert-update path with no extra call sites.
- [x] 3.3 PATCH path: patching `tss` to a value sets `tss_source='manual'`; patching `tss: null` clears both; PATCH never derives.
- [x] 3.4 Unit-test `deriveTSS` per formula and gate: power (2h @ IF 0.80 → 128), rTSS (270 vs 300 sec/km, 1h → 81), sTSS cubic (90 vs 100 sec/100m, 1h → 72.9), hrTSS (153/170, 1h → 81), LTHR preference, fall-through on unset thresholds, planned exclusion, IF > 2.5 skip, nil-config fail-open.

## 4. Handler integration tests (existing endpoints)

- [x] 4.1 POST /workouts: explicit `tss` with source garmin/manual stores the matching `tss_source`; derived power/pace/hr cases against a seeded athlete-config; no-method case returns 201 with both keys omitted; caller-sent `tss_source` key is ignored.
- [x] 4.2 POST /workouts/bulk: mixed batch derives per item (power + pace + none) with partial-failure semantics unchanged.
- [x] 4.3 Re-sync: first POST derives `pace`, re-POST of the same `external_id` with explicit `tss` flips to `garmin` (full-replace precedence re-applies).
- [x] 4.4 PATCH: `{"tss": 85}` → `tss_source='manual'`; `{"tss": null}` → both NULL; GET/list responses carry `tss_source` with omitempty.

## 5. Recompute endpoint

- [x] 5.1 Service method `RecomputeTSS(ctx)` iterating the candidate rows through `deriveTSS`, updating changed rows (including clearing to NULL when no method applies), returning `{examined, updated, by_source}` counts.
- [x] 5.2 Handler `POST /workouts/recompute-tss` with swag annotations; register in `Register(rg)` (idempotency middleware applies as to other POSTs; no `httpserver` wiring change — athlete-config repo is already injected).
- [x] 5.3 Integration tests: fills historical NULL rows; never touches `garmin`/`manual` rows; threshold change updates a `pace` row; cleared thresholds reset a computed row to NULL (`by_source.none`); empty run returns `updated: 0`.

## 6. MCP

- [x] 6.1 Add a `recompute_workout_tss` tool spec to `internal/agenttools/registry_workouts.go` (POST `/workouts/recompute-tss`, no args, write tool → idempotency key auto-derived). `AnnouncedToolNames` picks it up from the registry automatically.
- [x] 6.2 Regenerate the MCP schema golden (`go test ./internal/mcpserver/... -tags=goldengen`) and confirm the announced-schema and integration tests pass with the new tool; verify existing workout tools forward the wider body (with `tss_source`) verbatim (no schema change for them).

## 7. Docs and verification

- [x] 7.1 Run `task swag` to regenerate `docs/` (new endpoint + `tss_source` on the Workout shape); confirm both appear in `docs/swagger.json`.
- [x] 7.2 Run `task vet` and `go test -count=1 ./internal/workouts/... ./internal/agenttools/... ./internal/mcpserver/...`, then `task test`; re-run flaky testcontainers packages isolated with `-p 1` if needed.
- [x] 7.3 Propose the `feat(workouts): per-sport computed TSS with provenance` commit including this change directory, per the commit-after-apply convention.
