## Why

Phase 1 surfaced the Strava data the backend already had (records, gear, achievements, per-activity detail). The single most iconic Strava statistic is still missing: **volume totals** — "this week: 6 activities · 214 km · 11h 40m · 3,850 m ↑", by sport, over week / month / year-to-date, plus the calendar activity heatmap. Nothing aggregates workout distance/time/elevation today: `summarize()` in `coachcontext` does count/duration/kcal/by-sport for the dashboard lookback, and `/summary/*` is nutrition-only. This needs one new read-side backend rollup over the workouts table, then the frontend "training log" number wall.

## What Changes

- **New backend capability `workout-stats`** — a read-side aggregation over the `workouts` table in its own package (`internal/workoutstats/`), mirroring how `internal/summary` composes meals. No schema, no migration.
- **New endpoint `GET /api/v1/workouts/summary?from=&to=&tz=`** — returns per-day buckets (count, total duration, total distance, total elevation gain, total kcal, and a by-sport breakdown) plus a window total. The frontend derives week / month / YTD by choosing the range, and builds the heatmap from the per-day series. Follows the existing `/summary/range` param + tz + range-cap idiom, but with a **higher day cap (~400)** so YTD (up to 366 days) is a first-class range — nutrition's 92-day cap is too small.
- **MCP mirror `training_totals`** — one tool in `internal/agenttools/registry_workoutstats.go` issuing the single GET, per the repo's REST↔MCP 1:1 convention, so the coach agent can answer "how far have I ridden this month" without client math.
- **New frontend route `/stats`** (`react-router-dom`, added in Phase 1) — a period toggle (Week · Month · YTD) driving totals cards (distance, time, elevation, count, by-sport) and a GitHub-style activity heatmap, all in the existing analyst idiom (visx, `Panel`/`Stat`, muted). Header nav gains a Stats link.
- Units stay isolated per the repo convention: workout distance/elevation/duration live only in this new response shape and are **never merged** into any nutrition/hydration/energy totals. Distance and elevation are metres on the wire (frontend renders km), nullable workout metrics are summed as "present-only" so a window with unmeasured distance is not silently zeroed.

Out of scope (deferred): power/pace curve and best-power-per-duration — Phase 3, still gated on per-second stream ingestion the backend does not do. Brick/multisport per-leg distance attribution — the by-sport breakdown counts a multisport session as its own `multisport` bucket for distance/elevation (splitting a brick's distance across legs needs segment data); count/duration follow the same single-session treatment as `summarize()` today.

## Capabilities

### New Capabilities
- `workout-stats`: read-side aggregation of completed workouts into per-day + windowed totals (count, duration, distance, elevation, kcal) with a by-sport breakdown, exposed at `GET /workouts/summary` and mirrored as the `training_totals` MCP tool.

### Modified Capabilities
- `coach-dashboard`: adds a `/stats` route presenting week/month/YTD volume totals and an activity heatmap sourced from `GET /workouts/summary`, under the existing analyst aesthetic; header nav gains a Stats link.

## Impact

- **New backend package** `internal/workoutstats/` (`types.go`, `service.go`, `handlers.go`, tests) depending read-only on the `workouts` repo; wired in `internal/httpserver/server.go`; route `GET /workouts/summary`. Swag annotations → **`task swag` required**.
- **New MCP tool** `training_totals` in `internal/agenttools/registry_workoutstats.go`; bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`.
- **No migration** — pure composition over existing `workouts` columns (`started_at`/`ended_at`, `distance_m`, `elevation_gain_m`, `kcal_burned`, `sport`).
- **Frontend** (`apps/web/`): new `/stats` route + `StatsView`, totals cards, activity-heatmap component (visx), `useWorkoutStats(from,to)` hook, `Header` nav link.
- **API surface added:** `GET /api/v1/workouts/summary`. **New MCP tool:** `training_totals`.
