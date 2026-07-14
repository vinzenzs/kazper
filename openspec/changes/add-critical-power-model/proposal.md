## Why

The GoldenCheetah gap analysis (2026-07-14) surfaced critical-power modeling as the highest-value analytical gap — and the cheapest to close: the mean-maximal best-efforts ladder it fits over is already persisted (`effort-analytics`, since `strava-stats-frontend-phase-3`), and semi-automatic CP detection is a GC 3.7 headline feature. Today Kazper's FTP is whatever the athlete (or Garmin) last wrote into `athlete-config`; there is no data-derived estimate of CP/W′ to sanity-check it against, so a stale threshold silently skews TSS, pacing bands, zones, and the PMC. A compute-on-read CP/W′ fit gives the coach a evidence-based "your threshold looks ~10 W stale" conversation — the same posture as `add-performance-management`: pure math over existing rows, no migration.

## What Changes

- `GET /api/v1/workouts/cp-model?from=&to=&tz=` — fits the 2-parameter critical power model (work–time form, linear least squares) to the windowed per-duration best efforts (bike/power only in v1), returning `cp_watts`, `w_prime_kj`, fit quality (`r_squared`, `rmse_w`), and the contributing points (duration, watts, workout id, date).
- Fit inputs restricted to the model's validity band (efforts between 2 and 30 minutes); insufficient data degrades honestly to `200` with a null model + machine-readable `reason`, never a 5xx/422.
- **Advisory only**: the endpoint never reads or writes `athlete-config` — CP≈FTP interpretation and the compare-against-configured-threshold step stay with the consumer (coach agent / dashboard, which already have the configured FTP).
- New `cp_model` MCP tool (read tier, one GET, verbatim body).
- `/stats` dashboard panel: CP / W′ readout + effort points with the fitted curve, degraded state when the model is null.
- No migration, no new package (lands in `internal/effortanalytics/`), no change to ingest or existing endpoints.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `effort-analytics`: 2 ADDED requirements — the windowed CP/W′ model endpoint, and the `cp_model` MCP tool.
- `coach-dashboard`: 1 ADDED requirement — the `/stats` critical-power panel.

## Impact

- **Code:** `internal/effortanalytics/` gains a pure `cpmodel.go` (fit + gates) + handler; reuses the existing windowed best-effort MAX projection (widened if needed). `apps/web` gains a `/stats` panel + hook + types.
- **API/MCP:** one new GET; one new read MCP tool (registry-derived, golden regen additive). `task swag` required.
- **Dependencies/systems:** none new; no migration (head stays `059`); dataexport untouched (no new table).
- **Out of scope (deferred):** W′bal per-workout depletion (needs this change's CP/W′ as parameters — the natural follow-up), run/swim critical speed, the 3-parameter Morton model, any auto-apply into `athlete-config`/threshold-history.
