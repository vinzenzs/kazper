## Context

Phase 1 added routing and surfaced the already-exposed Strava data. Phase 2 adds the one thing that has no backend support: **volume totals**. The nearest precedent is `summarize()` in `internal/coachcontext/service.go`, which already rolls a workout window into `count` / `total_duration_min` / `total_kcal` / `by_sport` for the dashboard lookback — but only count per sport, no distance or elevation, and only a single window (not per-day, not period-bucketed). The nutrition analogue is `internal/summary` (`/summary/range`), a dedicated package composing meals into per-day + windowed totals with `from`/`to`/`tz` params and a `maxRangeDays` cap.

All the data exists on the `workouts` row: `started_at`/`ended_at` (duration), `distance_m`, `elevation_gain_m`, `kcal_burned`, `sport`, `status`. Phase 2 is a read-side composition — no schema, no migration.

## Goals / Non-Goals

**Goals:**
- One new read endpoint aggregating completed workouts into per-day + windowed totals with a by-sport breakdown.
- Support week / month / **year-to-date** ranges (the iconic Strava periods).
- A frontend `/stats` training-log view: period toggle + totals cards + activity heatmap.
- Mirror the endpoint as one MCP tool, per the repo's REST↔MCP 1:1 convention.

**Non-Goals:**
- No power/pace curve or best-power-per-duration (Phase 3, gated on stream ingestion).
- No per-leg brick/multisport distance attribution (a multisport session is one `multisport` by-sport entry).
- No new schema, migration, or write path.
- No merging of workout distance/elevation into nutrition/hydration/energy totals (unit isolation).
- No celebratory visual treatment.

## Decisions

### New package `internal/workoutstats/`, not an extension of `workouts` or `summary`
Mirror the `internal/summary` shape (`types.go` / `service.go` / `handlers.go` / tests) in a new package depending read-only on the `workouts` repo, wired in `httpserver.Run()`.

- **Why its own package:** it's a distinct capability with its own response shape and its own MCP tool, exactly like `summary` composes meals in its own package. Putting it in `workouts` would bloat the CRUD package; putting it in `summary` would cross nutrition and training units in one place — the opposite of the repo's unit-isolation rule.
- **Alternative — reuse `coachcontext.summarize()`:** rejected as the primary home; that function is dashboard-lookback-shaped (single window, count-only per sport). The new package MAY factor shared summing later, but the endpoint owns the richer per-day + distance/elevation shape.

### Response shape: per-day series + window total, mirroring `/summary/range`
`GET /workouts/summary?from=&to=&tz=` returns `{ from, to, tz, days: [DayBucket…], total: WindowTotal }`, where each `DayBucket`/`WindowTotal` carries `count`, `total_duration_min`, `total_distance_m`, `total_elevation_gain_m`, `total_kcal`, `by_sport`. The frontend picks the range for Week/Month/YTD and builds the heatmap from `days`.

- **Why per-day + total over pre-bucketed week/month/YTD objects:** one flexible shape serves all three toggles and the heatmap; the alternative (three fixed period objects) bakes UI choices into the API and still needs a per-day series for the heatmap. The per-day shape is also the direct analogue of `/summary/range`, so param parsing, tz resolution, and error contract are copy-shaped.
- **`by_sport`:** a `map[string]int` of count per sport (matching `LoadSummary.BySport`) is the minimum; distance/elevation per sport MAY be added if the totals cards want per-sport distance. Start with count-per-sport to match precedent; note per-sport distance as an easy extension.

### Higher day cap than nutrition's 92
`summary` caps ranges at `maxRangeDays = 92`. YTD can be 366 days. `workoutstats` uses its own cap (~400) — per-day workout aggregation is cheap (far fewer rows than meals) and YTD is a first-class use here, not an edge case.

### Present-only summation of nullable metrics
`distance_m`, `elevation_gain_m`, `kcal_burned` are nullable. A workout missing one contributes nothing to that sum (not zero-into-the-day). This mirrors how EA excludes workouts missing `kcal_burned` and keeps "unmeasured" honest rather than silently deflating totals.

### MCP tool `training_totals`
Add `internal/agenttools/registry_workoutstats.go` issuing the single `GET /workouts/summary`; bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`. Per the 1:1 convention, a new agent-useful read endpoint gets a tool ("how far have I ridden this month").

### Frontend: `/stats` route, visx heatmap, analyst idiom
`StatsView` with a Week/Month/YTD toggle driving `useWorkoutStats(from, to)` (React Query, `SLOW_INTERVAL_MS` backstop). Totals cards reuse `Stat`/`Panel`; the activity heatmap is a visx calendar grid (color scale by daily duration or count), consistent with the existing `LoadTrend` visx usage. Distance/elevation rendered km from metres. Header nav gains Stats.

## Risks / Trade-offs

- **[YTD per-day payload size]** → ~366 small day rows is trivially small JSON; the ~400 cap bounds it. No pagination needed.
- **[Multisport distance attribution is coarse]** → A brick counts as one `multisport` bucket, so its distance is not split run/bike/swim. Accepted for Phase 2 (splitting needs segment data + a resolver like `coachcontext.segmentSportResolver`); documented as a non-goal, revisitable.
- **[Timezone bucketing at day boundaries]** → A workout near midnight lands in the tz-local day; reuse the exact tz-resolution + day-bucketing approach `summary` already uses so behavior is consistent across the two rollups.
- **[Duration = elapsed (`ended_at − started_at`), not moving time]** → The backend stores no moving-time field, so totals are elapsed time. Matches `summarize()` today; label the frontend card accordingly ("time", not "moving time") to avoid implying Strava's moving-time semantics.
- **[Two rollups now compute by-sport]** → `coachcontext.summarize()` and `workoutstats` both aggregate by sport. Accepted duplication for now (different shapes); factor a shared helper only if a third consumer appears.

## Migration Plan

Additive; no data migration.
1. Backend: create `internal/workoutstats/` (types/service/handlers/tests); add a repo aggregation (either sum in Go over `workouts.List(from,to,…,status=completed)` or a dedicated SQL rollup — start with in-Go summation over the existing `List` for simplicity, optimize to SQL only if needed); wire route in `httpserver.Run()`; run `task swag`.
2. MCP: add `registry_workoutstats.go`; bump the integration expected-tools list.
3. Frontend: `useWorkoutStats` hook + types; `StatsView` with toggle, totals cards, visx heatmap; `/stats` route + header nav.
4. Tests: Go handler/service integration tests (range, tz, present-only sums, planned-excluded, error contract); vitest for `StatsView` (render, toggle, empty-state).
- **Rollback:** revert; no schema to unwind.

## Open Questions

- **By-sport granularity:** count-per-sport only, or also distance/elevation per sport in `by_sport`? Leaning count-only for parity with `LoadSummary`; add per-sport distance if the totals cards demand it during apply.
- **Aggregation site:** sum in Go over `Repo.List` vs a dedicated SQL `GROUP BY` rollup. Start in-Go (simplest, reuses `List`); move to SQL only if YTD latency warrants — flag during apply with real data.
- **Heatmap metric:** color the calendar by daily duration, count, or distance? Duration is the most sport-neutral; decide with the frontend during apply.
