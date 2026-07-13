# Tasks — add-performance-management

## 1. internal/pmc package (compute-on-read core)

- [ ] 1.1 Create `internal/pmc/types.go`: `Series {From, To, TZ, SeedDate *string, Days []Day, RampAlerts []RampAlert, MissingTSSWorkouts int}`, `Day {Date, TSSTotal, CTL, ATL, TSB, RampRate float64, MissingTSSCount int \`json:"missing_tss_count,omitempty"\`}`, `RampAlert {WeekStart, CTLStart, CTLEnd, CTLDelta}`; package constants `ctlTimeConstantDays = 42`, `atlTimeConstantDays = 7`, `rampAlertThreshold = 8.0`, `maxWindowDays = 400`.
- [ ] 1.2 Create `internal/pmc/repo.go` (read-only, against `store.Querier`): `EarliestCompletedDate(ctx, tz)` and `DailyTSS(ctx, tz, through)` returning per-local-day `{date, SUM(tss), COUNT(*) FILTER (WHERE tss IS NULL)}` over `status='completed'` workouts, bucketed by `(started_at AT TIME ZONE $tz)::date` (start-day attribution).
- [ ] 1.3 Create `internal/pmc/service.go`: param validation with sentinel errors mapping 1:1 to `range_required` / `date_invalid` / `range_invalid` / `range_too_large` / `tz_invalid`; the pure EWMA pass from `seed_date` (ctl=atl=0) through `to` (full precision internally); TSB from previous day's CTL/ATL; per-day `ramp_rate = ctl(d) − ctl(d−7)` with zero baseline before the seed; Monday-start weekly `ramp_alerts` where `ctl_delta > 8.0`; window-level `missing_tss_workouts`; slice to `[from, to]`.
- [ ] 1.4 Unit-test the pure math (no DB): EWMA recurrence values against hand-computed fixtures; TSB uses yesterday; rest-day decay; window independence (same date, two windows, identical values); seed/zero-history behavior; ramp-rate zero baseline; ramp-alert boundary (delta exactly 8.0 → no alert, 8.1 → alert); rounding applied only at the boundary.
- [ ] 1.5 Create `internal/pmc/handlers.go`: `GET /performance/pmc` with swag annotations, `tz` defaulting to `DEFAULT_USER_TZ` and echoed, `numfmt.Round1` on all numeric outputs, `Register(rg *gin.RouterGroup)`.

## 2. HTTP wiring

- [ ] 2.1 Wire the package in `internal/httpserver/server.go` `Run()`: instantiate `pmc.NewService(pmc.NewRepo(pool))` and register the handlers (follow the `effortanalytics` wiring shape, passing `cfg.DefaultUserTZ`).
- [ ] 2.2 Integration-test the endpoint against testcontainers Postgres (`internal/pmc/handlers_test.go`): populated window series shape + ordering; planned workouts excluded; NULL-TSS completed workout → `tss_total` unaffected, `missing_tss_count: 1`, window `missing_tss_workouts` bumped, key omitted on full-coverage days; timezone bucketing across midnight; empty-history all-zero series with `200 OK` and empty `ramp_alerts`; each 400 validation code; `Idempotency-Key` on GET ignored (no cache row).

## 3. MCP tool

- [ ] 3.1 Add `internal/agenttools/registry_pmc.go`: `pmc_series` spec (`from`, `to`, optional `tz`; `TierRead`; builds one `GET /performance/pmc` call; no idempotency key) with a description naming CTL/ATL/TSB semantics and distinguishing it from the Garmin acute/chronic mirror; registry unit test alongside (`registry_pmc_test.go`) per the sibling registry tests.
- [ ] 3.2 Regenerate the announced-schema golden (`go test -tags=goldengen ./internal/mcpserver/...` capture path) so `internal/mcpserver/testdata/announced_schemas.json` includes `pmc_series`, then run the MCP integration test (`-tags=integration`) — the expected-tools assertion derives from the registry, so it must pass with the new tool announced.

## 4. Dashboard chart

- [ ] 4.1 Add the `PMCSeries` response type to `apps/web/src/api/types.ts` and a `usePMC(from, to)` hook in `apps/web/src/api/hooks.ts`.
- [ ] 4.2 Build the PMC panel on the `/stats` surface: CTL line + ATL line + TSB area/bars around a zero baseline (visx, existing `Panel`/`Stat` idiom), 90/180/365-day window selector wired to the query range, ramp-alert weeks highlighted on the CTL trace, empty-state for all-zero history.
- [ ] 4.3 Add/extend the view test under `apps/web/src/views/__tests__/` (render with fixture series, selector re-query, empty-state), run the web test suite, and rebuild the committed bundle via `task web:build`.

## 5. Docs & verification

- [ ] 5.1 Run `task swag` to regenerate `docs/` for the new endpoint (verify `/performance/pmc` and the `Series`/`Day`/`RampAlert` shapes appear in `docs/swagger.json`).
- [ ] 5.2 Run `task vet` and `go test -count=1 ./internal/pmc/...` plus the MCP integration test; then `task test` (re-run any testcontainers parallel-boot flakes isolated with `-p 1`).
