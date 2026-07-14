# Tasks — add-race-pacing-plan

## 1. Migration

- [x] 1.1 `task migrate:new NAME=add_race_leg_pacing_overrides` (verify the next free number — head is `054_sync_run_summary_partial` at proposal time; out-of-band slots have happened).
- [x] 1.2 Up migration: create `race_leg_pacing_overrides` (`race_id UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE, ordinal INT NOT NULL, target_power_low_w INT NULL, target_power_high_w INT NULL, target_pace_low_sec_per_km NUMERIC NULL, target_pace_high_sec_per_km NUMERIC NULL, target_pace_low_sec_per_100m NUMERIC NULL, target_pace_high_sec_per_100m NUMERIC NULL, note TEXT NULL, created_at/updated_at TIMESTAMPTZ, PRIMARY KEY (race_id, ordinal)`) with CHECKs: exactly one unit family fully populated (low AND high), `low <= high`, values `> 0`.
- [x] 1.3 Down migration: drop `race_leg_pacing_overrides`.

## 2. Package scaffolding (`internal/racepacing/`)

- [x] 2.1 `types.go`: `PacingPlan` / `LegPacingPlan` (per-leg `source`, discipline-appropriate `target_*` fields with `omitempty`, `intensity_factor`, `estimated_tss`, `missing_thresholds`, `rationale`; race-level `estimated_tss_total`, `tss_complete`, `missing_thresholds` union, `total_duration_min`) and `Override` mirroring the new row.
- [x] 2.2 `repo.go`: override CRUD against `store.Querier` — `UpsertOverride` (full-replace on the `(race_id, ordinal)` key), `GetOverridesForRace`, `DeleteOverride` — usable from `*pgxpool.Pool` or `pgx.Tx`.

## 3. Pure pacing math (`compute.go`, unit-tested without the DB)

- [x] 3.1 Bike bands (% FTP by leg duration: `<45 → 90–100`, `[45,90) → 83–90`, `[90,180) → 75–83`, `≥180 → 68–78`); `target_power_*_w = round(ftp × band)` integer watts; `IF = band midpoint`; `estimated_tss = duration_hr × IF² × 100`. Cite sources next to the constants.
- [x] 3.2 Run bands (threshold-pace multipliers by leg duration: `<30 → 1.00–1.04`, `[30,60) → 1.04–1.10`, `[60,150) → 1.10–1.18`, `≥150 → 1.18–1.28`); `IF = 1/mult_mid`; `estimated_tss = duration_hr × IF² × 100`; rationale notes the off-the-bike context when a lower-ordinal bike leg exists.
- [x] 3.3 Swim bands (CSS multipliers: `<20 → 1.00–1.05`, `[20,45) → 1.03–1.08`, `≥45 → 1.06–1.12`); `IF = 1/mult_mid`; `estimated_tss = duration_hr × IF³ × 100` (sTSS convention).
- [x] 3.4 Degradation paths: transition legs (no target, TSS 0), `other` legs and missing-threshold legs (`source: none`, per-leg + race-level `missing_thresholds`, rationale), unknown-duration legs; `tss_complete` false whenever a swim/bike/run leg lacks an estimate; `estimated_tss_total` sums only computed legs.
- [x] 3.5 Override merge: matching-family override wins (`source: "override"`, IF/TSS re-derived from the override midpoint when the threshold is set); family/discipline mismatch after a leg edit is ignored with a rationale note; ordinals with no current leg are not surfaced.
- [x] 3.6 Pure-function tests: every band boundary per discipline (44/45, 89/90, 179/180 min bike; 29/30, 59/60, 149/150 run; 19/20, 44/45 swim), exact scenario numbers from the spec (FTP 265 × 300 min → 180–207 W / IF 0.73 / TSS 266.5; threshold 270 × 100 min → 297.0–318.6; CSS 105 × 70 min → 111.3–117.6 / TSS 90.1), missing-threshold and no-config cases, override merge + mismatch-ignore, totals honesty.

## 4. Service + validation (`service.go`)

- [x] 4.1 Sentinel errors mapping 1:1 to API codes: `race_not_found` (reuse the races repo's not-found), `leg_not_found`, `override_discipline_mismatch`, `override_target_required`, `override_unit_conflict`, `override_band_invalid`.
- [x] 4.2 `Plan(ctx, raceID)`: load race + legs via the injected `races` repo, thresholds via the `athleteconfig` repo (absent row = all thresholds unset, still 200), overrides via own repo; call the pure compute.
- [x] 4.3 `SetOverride(ctx, raceID, ordinal, input)`: validate race + leg existence, exactly-one unit family, family matches the leg's discipline (transition/`other` always mismatch), positive finite values, `low ≤ high`; full-replace upsert. `DeleteOverride` with `override_not_found` when absent.

## 5. Handlers + routes + swag (`handlers.go`)

- [x] 5.1 `GET /races/{id}/pacing-plan` — 404 on unknown race, `numfmt` rounding at the boundary (watts integer, paces/TSS `Round1`, IF `Round2`).
- [x] 5.2 `PUT /races/{id}/pacing-plan/overrides/{ordinal}` (full-replace; the idempotency middleware's PUT rule yields `400 idempotency_unsupported_for_put` when the header is present) and `DELETE /races/{id}/pacing-plan/overrides/{ordinal}` (204; 404 `override_not_found`).
- [x] 5.3 Map every sentinel to its documented status + code; swag annotations on all handlers including the error codes; `Register(rg *gin.RouterGroup)` per convention.
- [x] 5.4 Per-handler integration tests against testcontainers Postgres: plan happy path (full tri with all thresholds), missing-FTP partial response (bike leg degraded, run leg intact, race-level union), no-config-row 200, unknown race 404, override PUT→plan round-trip (`source: "override"`), DELETE revert, override survives a legs-replacing race PATCH, discipline-mismatch 400 at write + ignore-at-read after a leg swap, `leg_not_found` 404, band-invalid 400, Idempotency-Key-on-PUT 400, and `NotContains` unit-isolation guards (bike leg JSON has no `sec_per_km`/`sec_per_100m`; plan has no merged target struct).

## 6. Wiring

- [x] 6.1 `internal/httpserver/server.go`: instantiate the racepacing repo/service/handlers with the pool, `racesRepo`, and `athleteConfigRepo` (the `workoutfueling` multi-repo pattern) and register routes inside the authed API group.

## 7. MCP tools (shared registry)

- [x] 7.1 `internal/agenttools/registry_racepacing.go`: `plan_race_pacing` (TierRead → `GET /races/{id}/pacing-plan`), `set_race_leg_pacing_override` (TierWriteConfirm → PUT; the dispatcher skips the idempotency header on PUT centrally — Build must not add one), `clear_race_leg_pacing_override` (TierWriteConfirm → DELETE). Each builds exactly one `HTTPCall`; register via `registerMCPDomain`.
- [x] 7.2 Registry unit tests for path/body construction (mirroring `registry_goaloverrides_test.go`); regenerate the MCP schema golden (`go test -tags=goldengen ./internal/mcpserver/...`) and confirm the registry-equality assertion in `internal/mcpserver/mcp_integration_test.go` passes (announced surface is registry-derived — no manual expected-tools list edit).

## 8. Docs

- [x] 8.1 `task swag` to regenerate `docs/` after the handler changes.
- [x] 8.2 README MCP tool table: add the three pacing tool rows.
- [x] 8.3 RUN_LOCAL.md: short walkthrough — set thresholds via `PUT /athlete-config`, then `GET /races/{id}/pacing-plan`, then pin an override and re-read.

## 9. Verification

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green (new package + MCP integration tag).
- [x] 9.3 `openspec validate add-race-pacing-plan --strict` passes.
- [x] 9.4 If `add-per-sport-tss` has archived by apply time, re-base the athlete-config MODIFIED block on the merged main-spec text and align TSS-estimate constants with its rTSS/sTSS formulas.
