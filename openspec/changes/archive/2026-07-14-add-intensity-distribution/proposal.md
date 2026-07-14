# Add an Intensity-Distribution Report (Time in Zone)

## Why

Kazper stores per-workout HR-zone time (`secs_in_zone_1..5`, synced from Garmin) but has no
aggregate view of it — the "how has my intensity been distributed over the last N weeks"
report every endurance coach uses to check polarized vs pyramidal vs threshold-heavy training.

## What Changes

- `workout-stats` (the read-side aggregation capability) gains a second windowed endpoint,
  `GET /workouts/intensity-distribution?from=&to=&tz=`, aggregating time in zone over
  completed workouts: total seconds and percentage share per zone (1–5) for the window, a
  by-sport breakdown, and a Monday-start per-week trend series (the adherence `weekly`
  idiom). Compute-on-read over the existing `workouts` rows — no new tables, no migration.
- A deterministic, documented **distribution classification** for coach consumption:
  the five zones collapse into three bands (low = Z1+Z2, moderate = Z3, high = Z4+Z5) and a
  simple share heuristic labels the window `polarized` / `pyramidal` / `threshold` / `mixed`
  (null when the window has no zone time). The band shares are returned alongside the label
  so it is auditable, never a black box.
- Missing-data honesty per the `energy-availability` / PMC precedent: completed workouts with
  no zone data at all are excluded from the sums but surfaced via a window-level
  `missing_zone_data_count` (plus per-week counts), so a strength-heavy month can't
  masquerade as a polarized one.
- A secondary **session-count distribution by `training_focus`** (the stored 7-zone German
  classification): counts per focus plus an unclassified count — the planned-intent axis next
  to the measured time-in-zone axis, from the same repo read.
- New MCP tool `intensity_distribution` mirroring the endpoint 1:1 via the shared
  `agenttools` registry (one HTTP call, verbatim forward, read tier); the announced-schema
  golden is regenerated via `-tags=goldengen`.
- Coach-dashboard: the `/stats` surface gains an intensity panel — window zone-share bar with
  the classification badge and a per-week stacked zone-share trend.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `workout-stats` — gains the intensity-distribution aggregation endpoint, the
  classification, the training-focus axis, and the mirroring MCP tool (this spec already
  carries the capability's MCP requirement, per `training_totals`).
- `coach-dashboard` — the `/stats` surface gains an intensity-distribution panel
  requirement.

## Impact

- **Specs:** ADDED requirements on `workout-stats` and `coach-dashboard`. No MODIFIED
  requirements — the existing volume-summary behavior is untouched.
- **Code:** extend `internal/workoutstats/` (types / service / handlers — the new route
  registers inside the package's existing `Register`, so `internal/httpserver/server.go`
  needs no change); new tool entry in `internal/agenttools/registry_workoutstats.go` +
  regenerated MCP golden schema; intensity panel in `apps/web/src/` on the `/stats` route +
  rebuilt `apps/web/dist`.
- **No migration.** Reads existing `workouts` columns only (`status`, `started_at`, `sport`,
  `secs_in_zone_1..5`, `training_focus`). Read-only endpoint — never consumes an
  `Idempotency-Key`.
- **Docs:** `task swag` regenerates `docs/` for the new endpoint.
