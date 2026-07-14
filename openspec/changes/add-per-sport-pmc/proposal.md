## Why

Deferred from the PMC v1 and second-voted by Intervals.icu (per-discipline fitness tracking): a triathlete's combined CTL hides that bike fitness is climbing while run fitness decays. Per-sport TSS is already honest (`tss_source`, per-sport derivation), so filtering the PMC by sport is pure plumbing over shipped data.

## What Changes

- `GET /api/v1/performance/pmc` gains an optional `sport=` parameter (bike|run|swim|strength|multisport|…, the workouts sport vocabulary): the CTL/ATL/TSB EWMA computed over only that sport's completed-workout TSS — warm-up, `seed_date`, ramp alerts, and missing-TSS honesty all within the filtered series. Omitted → today's combined behavior, unchanged. Unknown value → `400 sport_invalid`. Response echoes `sport` when filtered.
- `pmc_series` MCP tool gains the optional `sport` arg (forwarded verbatim).
- The `/stats` PMC panel gains an All/Bike/Run/Swim selector.
- Multisport workouts count under `sport=multisport` (no per-segment TSS split exists — the workout-stats `by_sport` precedent), documented in the tool description.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `performance-management`: 1 ADDED requirement — the sport-filtered PMC (including the MCP arg forwarding).
- `coach-dashboard`: 1 ADDED requirement — the PMC sport selector.

## Impact

- **Code:** `internal/pmc/` repo query gains a sport predicate; service unchanged (same EWMA over a filtered series); `apps/web` selector.
- **API/MCP:** param + tool arg; golden regen (input-schema touch); `task swag`.
- **Out of scope:** per-segment multisport TSS attribution; per-sport target trajectories (composes later with `add-planned-vs-actual-ctl`); side-by-side multi-sport series in one response (call per sport).
