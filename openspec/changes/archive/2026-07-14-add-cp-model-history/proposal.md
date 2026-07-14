## Why

The CP model answers "what's my CP now"; the Intervals.icu-inspired follow-up is "how has it moved" — the data-derived counterpart to `threshold-history`'s record of *configured* FTP. A rolling-window fit series shows whether the season is actually raising the ceiling, and juxtaposed with threshold-history it shows how stale the configured FTP tends to run.

## What Changes

- `GET /api/v1/workouts/cp-model/history?from=&to=&tz=&window_days=` — fits the existing CP2 model at weekly (Monday) anchors across the range, each anchor over its trailing `window_days` (default 90, bounds [30, 365]); per anchor: the fitted model or `null` + the existing gate `reason`. Range cap 400 days.
- New `cp_model_history` MCP tool (read tier, one GET, verbatim).
- `/stats` CP panel gains a CP-over-time trend line (null anchors gapped, not zeroed), with the configured-FTP step line from `/athlete-config/history` overlaid client-side — the backend stays uncoupled from athlete-config (cp-model D4 holds).
- Pure reuse of the shipped fit + gates; compute-on-read, no migration.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `effort-analytics`: 2 ADDED requirements — the history endpoint and the MCP tool.
- `coach-dashboard`: 1 ADDED requirement — the CP trend with configured-FTP overlay.

## Impact

- **Code:** `internal/effortanalytics/` anchor loop over the existing fit (one windowed-MAX query per anchor); `apps/web` trend chart.
- **API/MCP:** one GET, one read tool, golden additive, `task swag`.
- **Out of scope:** persisting fit history, auto-applying thresholds (D4 stands), per-anchor W′ trend charting (returned but not charted v1).
