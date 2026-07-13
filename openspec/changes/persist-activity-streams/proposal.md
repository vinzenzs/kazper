# Persist activity streams and derive execution-quality metrics

## Why

`POST /workouts/{id}/streams` computes mean-maximal best-efforts in-request and then
discards the raw 1 Hz samples, so nothing can ever be re-analyzed when the logic or the
athlete's thresholds change, no interval graph can be drawn, and execution-quality
metrics (VI, EF, decoupling) that need the full time series are impossible.

## What Changes

- **Persist the raw streams.** A new `workout_streams` table stores one row per
  (workout, stream type) with the whole 1 Hz sample series as a native Postgres
  `REAL[]` column. Re-posting replaces; deleting the workout cascades.
- **Extend the ingest payload with a heart-rate series.** The bridge today extracts only
  `directPower` and `directSpeed` from Garmin activity detail; it is extended to also
  extract `directHeartRate` and post it as `heart_rate` (bpm). The backend accepts and
  persists it. Payload change is additive — NOT breaking (old two-series posts remain
  valid, response keeps `records_written` and gains an additive `streams_stored` count).
- **Derive execution metrics at ingest.** Variability Index (NP / avg power), Efficiency
  Factor (NP or avg speed per avg HR), and aerobic decoupling (first-half vs second-half
  output:HR drift %) are computed from the posted streams and stored as three new
  nullable columns on the `workouts` row — the same placement precedent as
  `intensity_factor` / `normalized_power_w`. They surface on `GET /workouts/{id}` with
  the usual omitempty rule.
- **Recompute endpoint.** `POST /workouts/{id}/streams/recompute` re-derives best-efforts
  and execution metrics from the *stored* streams — the re-analysis path the discard
  previously blocked. Ingest keeps computing on POST exactly as today.
- **Retrieval endpoint.** `GET /workouts/{id}/streams` returns the stored series, with an
  optional `downsample=<points>` bucket-mean reduction for dashboard graphs.
- **MCP:** one new `recompute_workout_streams` tool (single POST, tiny response). Raw
  stream retrieval is deliberately NOT exposed over MCP — a 10k+-sample array is
  pathological in an agent context and the streams POST already has no tool (bridge-only
  write path); the agent-relevant derivatives (execution metrics, best-efforts,
  power curve) are all MCP-reachable already. The parity exception is spec'd explicitly.

## Capabilities

### New Capabilities

- `activity-streams` — persistence of raw per-workout sample streams, the retrieval and
  recompute endpoints, and the derivation of stream-based execution metrics. Kept
  separate from `effort-analytics`, which remains the derived best-effort/curve
  capability (one package per capability; `internal/activitystreams` imports
  `effortanalytics` + `workouts`, no cycle).

### Modified Capabilities

- `effort-analytics` — the ingest requirement currently states raw streams are computed
  over in-request and not persisted; that clause is replaced (persistence now happens,
  owned by `activity-streams`) and best-efforts become recomputable from stored streams.
- `workouts` — three new nullable stream-derived execution-metric columns on the
  `workouts` row, echoed on reads (omitempty), not writable via PATCH.
- `garmin-bridge` — the per-activity stream sync requirement is extended to also extract
  and post the heart-rate series.

## Impact

- **Migration:** one new pair (`workout_streams` table + three `workouts` columns) —
  verify the current highest migration number before scaffolding (head is around `054`).
- **Code:** new `internal/activitystreams/` package (types/repo/service/handlers); the
  `POST /workouts/{id}/streams` route moves from `effortanalytics` handlers to
  `activitystreams` handlers, which persists and then delegates best-effort computation
  to the existing `effortanalytics` service; `internal/workouts/` echoes the new columns;
  `internal/httpserver/server.go` wiring.
- **Bridge:** `apps/garmin-bridge/garmin_bridge/mapping.py` (`_extract_streams`) +
  bridge tests (`test_mapping.py`, `test_effort_streams.py`, `test_sync.py`).
- **MCP:** new registry entry in `internal/agenttools/`, announced-schema golden
  regeneration (`-tags=goldengen`), MCP integration test re-run.
- **Storage:** ~130 KB per instrumented workout (3 streams × ~3 h × 4 B), ~65 MB/year
  for this single-user load before TOAST compression — no retention policy in v1.
- **Docs:** `task swag` for the changed/new handlers.
