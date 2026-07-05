## 1. Backend: workout-stats package

- [x] 1.1 Create `internal/workoutstats/types.go`: response shape (`from`/`to`/`tz`, `days: []DayBucket`, `total: WindowTotal`) with `count`, `total_duration_min`, `total_distance_m`, `total_elevation_gain_m`, `total_kcal`, `by_sport`
- [x] 1.2 Add a repo/aggregation path — start with in-Go summation over `workouts.Repo.List(from, to, …, status=completed)`; bucket by tz-local calendar day
- [x] 1.3 Create `internal/workoutstats/service.go`: params (from/to/tz + day cap ~400), present-only summation of nullable `distance_m`/`elevation_gain_m`/`kcal_burned`, multisport as a single `multisport` by-sport entry, `numfmt` rounding at the boundary
- [x] 1.4 Create `internal/workoutstats/handlers.go`: `GET /workouts/summary` with swag annotations and the `range_required` / `date_invalid` / `range_invalid` / `range_too_large` / `tz_invalid` error contract (mirror `summary` handler)
- [x] 1.5 Wire the package (repo/service/handler + route registration) in `internal/httpserver/server.go`
- [x] 1.6 Add integration tests: range totals + per-day series, present-only sums, planned excluded, YTD range accepted, full error contract
- [x] 1.7 Run `task swag` to regenerate `docs/`

## 2. MCP mirror

- [x] 2.1 Add `internal/agenttools/registry_workoutstats.go` with a `training_totals` tool issuing the single `GET /workouts/summary`
- [x] 2.2 Register the tool group in the MCP server and bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`

## 3. Frontend: stats route

- [x] 3.1 Add `WorkoutStats` response types in `api/types.ts` mirroring the Go shape
- [x] 3.2 Add `useWorkoutStats(from, to)` hook in `api/hooks.ts` (React Query + `SLOW_INTERVAL_MS` backstop)
- [x] 3.3 Build totals cards (distance km, time, elevation, count, by-sport) reusing `Stat`/`Panel`; render distance/elevation from metres
- [x] 3.4 Build an activity heatmap component (visx calendar grid, color scale by daily duration) from the per-day series
- [x] 3.5 Assemble `StatsView` with a Week · Month · YTD period toggle that selects the range, plus loading/error/empty-state handling
- [x] 3.6 Add the `/stats` route to the router and a Stats link to the header nav

## 4. Tests & build

- [x] 4.1 Vitest for `StatsView`: populated render, period-toggle switches the window, empty-state
- [x] 4.2 Run `apps/web` lint + typecheck + `vitest`; build the SPA and confirm the `webembed` serving tests still pass
- [x] 4.3 Run `task test` (or the workoutstats + mcpserver packages) and `task vet`
