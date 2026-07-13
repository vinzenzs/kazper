# Tasks — add-intensity-distribution

## 1. internal/workoutstats additions (compute-on-read core)

- [ ] 1.1 Extend `internal/workoutstats/types.go`: `Distribution {From, To, TZ, Total ZoneAggregate, BySport map[string]ZoneAggregate, Weekly []WeekBucket, ByTrainingFocus map[string]int, UnclassifiedFocusCount int, MissingZoneDataCount int}`, `ZoneAggregate {WorkoutsCounted, TotalZoneSecs int, Zones [5]ZoneShare (serialized as a 5-entry array ordered zone 1→5), Bands *Bands (Total only), Classification *string (Total only)}`, `ZoneShare {Zone int, Secs int, SharePct *float64 \`json:"share_pct,omitempty"\`}`, `Bands {LowPct, ModeratePct, HighPct float64}`, `WeekBucket {WeekStart string, ZoneAggregate, MissingZoneDataCount int \`json:"missing_zone_data_count,omitempty"\`}`; package constants `thresholdBandPct = 20.0`, `lowBaseBandPct = 75.0` (the existing `maxRangeDays = 400` is reused).
- [ ] 1.2 Add `DistributionFor(ctx, Params)` to `internal/workoutstats/service.go`: one `repo.List(fromDay, upper, nil, &completed)` read (same window expansion as `SummaryFor`); a workout counts when ≥1 `SecsInZone*` is non-null (null zones contribute 0), all-null workouts feed only the missing counters; accumulate window total, `by_sport` (`string(w.Sport)`, multisport as one entry), Monday-start tz-local weekly buckets (emitted only for weeks with ≥1 completed workout, missing-only weeks included, no zero-fill), `by_training_focus` counts + `unclassified_focus_count`; shares/bands at full precision, `numfmt.Round1` only at the boundary, `share_pct` omitted when a group's `total_zone_secs` is 0.
- [ ] 1.3 Implement the pure band-collapse + classification helpers (low=Z1+Z2, moderate=Z3, high=Z4+Z5; null / threshold / polarized / pyramidal / mixed per the design's total partition) and unit-test them without a database: each label branch, boundary values (moderate exactly 20.0 → threshold; low exactly 75.0; high == moderate at the polarized/pyramidal split), zero-zone-time → null, shares summing to ~100 after rounding.
- [ ] 1.4 Add the handler to `internal/workoutstats/handlers.go`: `GET /workouts/intensity-distribution` with swag annotations, reusing the existing `from`/`to`/`tz` parsing and `range_required` / `date_invalid` / `range_invalid` / `range_too_large` (`max_days: 400`) / `tz_invalid` responses, registered inside the package's existing `Register(rg *gin.RouterGroup)` — verify no `internal/httpserver/server.go` change is needed (the package is already wired).

## 2. Integration tests (testcontainers Postgres)

- [ ] 2.1 Extend `internal/workoutstats/handlers_test.go` with the distribution endpoint: populated window → 5-entry ordered `zones` with shares summing to ~100, `by_sport` split, weekly buckets on Monday-start weeks with edge weeks clipped to the window and empty weeks absent; planned workouts excluded; per-workout null individual zones summed present-only.
- [ ] 2.2 Missing-zone-data honesty: an all-null-zones completed workout excluded from sums but counted (window `missing_zone_data_count`, per-week omitempty counter, `workouts_counted + missing == completed count`); a week containing only zone-less workouts still emits a bucket; full coverage → window count 0 and no per-week keys.
- [ ] 2.3 Classification through the API: fixtures driving each label (polarized / pyramidal / threshold / mixed) plus zone-less window → `classification` null, `share_pct` omitted, `200 OK`; `bands` present and 1dp-rounded; no label on `weekly`/`by_sport` entries.
- [ ] 2.4 Training-focus axis: annotated + unannotated fixtures → `by_training_focus` counts and `unclassified_focus_count`; annotations do not alter the measured classification.
- [ ] 2.5 Timezone bucketing (late-UTC workout landing on the next local day/week under `Europe/Berlin`), each 400 validation code, and unit isolation (`assert.NotContains` for nutrition/hydration keys in the distribution body and for zone-share keys in `/workouts/summary`).

## 3. MCP tool

- [ ] 3.1 Add the `intensity_distribution` tool to `internal/agenttools/registry_workoutstats.go` (`from`, `to`, optional `tz`; `TierRead`; builds one `GET /workouts/intensity-distribution` call; no idempotency key) with a description naming the zone-share + classification semantics and pointing volume questions at `training_totals`; extend `registry_workoutstats_test.go` per the sibling registry tests.
- [ ] 3.2 Regenerate the announced-schema golden via `go test -tags=goldengen ./internal/mcpserver/...` so `internal/mcpserver/testdata/announced_schemas.json` includes `intensity_distribution`, then run the MCP integration test (`-tags=integration`) — the announced surface is registry-derived, no hand-maintained tool list is edited.

## 4. Dashboard panel

- [ ] 4.1 Add the `IntensityDistribution` response types to `apps/web/src/api/types.ts` and a `useIntensityDistribution(from, to)` hook in `apps/web/src/api/hooks.ts`.
- [ ] 4.2 Build the intensity panel on the `/stats` surface (visx, existing `Panel`/`Stat` idiom): window zone-share bar with the classification badge + band shares, per-week stacked zone-share bars from `weekly`, muted `missing_zone_data_count` note, wired to the route's existing period selection, empty-state for zone-less windows.
- [ ] 4.3 Add/extend the view test under `apps/web/src/views/__tests__/` (render with fixture distribution, period re-query, missing-data note, empty-state), run the web test suite, and rebuild the committed bundle via `task web:build`.

## 5. Docs & verification

- [ ] 5.1 Run `task swag` to regenerate `docs/` (verify `/workouts/intensity-distribution` and the `Distribution`/`ZoneShare`/`Bands` shapes appear in `docs/swagger.json`).
- [ ] 5.2 Run `task vet` and `go test -count=1 ./internal/workoutstats/...` plus the MCP integration test; then `task test` (re-run any testcontainers parallel-boot flakes isolated with `-p 1`).
