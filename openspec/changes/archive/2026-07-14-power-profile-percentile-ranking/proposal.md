## Why

The mean-maximal power curve (`add-strava-stats` Phase 3) and the critical-power model (`add-critical-power-model`) both answer *what power the athlete can hold* — but neither answers *how good that is*. The natural next read over the same stored best-efforts is a **power-profile ranking**: Coggan's power-profile tables map W/kg at four benchmark durations (5 s neuromuscular, 1 min anaerobic, 5 min VO₂max, ~threshold) onto named ability categories (untrained → world-class) and imply a rider phenotype (sprinter / time-trialist / climber / all-rounder). It's the cheapest remaining GoldenCheetah gap to close: no migration, no new streams, a pure compute-on-read table lookup over the ladder the power curve already builds. It also seeds the deferred v2 `progress` number for the public race site.

## What Changes

- New read `GET /api/v1/workouts/power-profile?from=&to=&tz=&weight_kg=&sex=`: reuse `CurveFor` (power metric) to pull the windowed best at the four Coggan anchor durations (5 s / 1 min / 5 min / 20 min-as-FT-proxy), divide by body weight, and rank each W/kg against the embedded Coggan table for the athlete's sex.
- Per anchor: `watts`, `w_per_kg`, the Coggan `category` band, an interpolated `percentile` (0–100, explicitly an estimate), and the contributing workout/date. Plus an advisory `phenotype` derived from the relative category strength across the four anchors (null unless all four are present).
- **Weight resolution** (the energy-endpoint precedent): `weight_kg` query param → else the most-recent stored body-weight entry → else `400 weight_data_missing`; the response echoes `weight_kg` + `weight_source`.
- `sex` selects the Coggan table (`male` default, `female`); other values `400 sex_invalid`. Power only (no run/swim Coggan equivalent).
- MCP `power_profile` read tool (one GET, body verbatim). `/stats` dashboard panel: the four anchors with category badges + the phenotype label.
- **Advisory only** — like the CP model, this endpoint never reads or writes athlete-config; the category tables are a fixed reference, not a personal calibration.

## Capabilities

### Modified Capabilities

- `effort-analytics`: ADD the power-profile ranking endpoint (Coggan category + interpolated percentile + phenotype over the windowed best-efforts) and its `power_profile` MCP tool.
- `coach-dashboard`: ADD a power-profile panel on `/stats`.

## Impact

- **Code:** extend `internal/effortanalytics/` (new `powerprofile.go` pure ranking + embedded Coggan tables, service method reusing `CurveFor`, handler sharing `parseWindow`); a narrow injected `weightProvider` (latest body-weight) wired in `httpserver.Run()`; MCP registry + golden (additive); `apps/web` panel + hook + types.
- **API/MCP:** one REST route, one MCP tool. `task swag` required.
- **No migration, nothing persisted** (compute-on-read).
- **Out of scope (deferred):** run critical-speed / swim ranking, a configurable/custom reference table, persisting a ranking history or trend, wiring the phenotype into `/context/training`, and the public-site v2 `progress` number (this endpoint is a building block for it, not the number itself).
