## Why

The last gap-analysis item needing schema work, deliberately sequenced last: quadrant analysis (GoldenCheetah's force/velocity view) shows *how* power is produced — grinding high-force/low-cadence vs spinning — which informs cadence prescriptions and race-position work. It needs a cadence stream Kazper doesn't store: migration 056's `workout_streams` CHECK admits only `power`/`speed`/`heart_rate`. This change widens the stream pipeline for `cadence` end to end (bridge → ingest → storage → retrieval) and builds the per-workout quadrant read on top.

## What Changes

- **Migration (next free slot):** widen the `workout_streams.stream_type` CHECK to include `'cadence'` (rpm as REAL samples; storage semantics unchanged).
- `activity-streams` ingest accepts an optional `cadence` array (gap-filled zeros like HR, all-non-positive dropped); retrieval serves it under `streams.cadence`; cadence feeds no best-effort, no execution metric, no nutrition/energy total.
- garmin-bridge: defensive `directBikeCadence` extraction (the `directPower`/`directSpeed` pattern — unexpected shape → no cadence, never a crash), posted with the existing stream POST.
- `GET /api/v1/workouts/{id}/quadrant?cp_watts=&cadence_rpm=&crank_mm=` — computes per-sample AEPF/CPV from paired power+cadence samples, splits at the force/velocity point implied by (`cp_watts`, `cadence_rpm`; crank default 172.5 mm), returns quadrant time shares + counts and a downsampled scatter (`summary_only` supported). CP and pivot cadence are explicit params (the W′bal convention).
- New `quadrant_analysis` MCP tool (read tier, always `summary_only=true` — the scatter is chart data).
- Workout-detail dashboard gains a quadrant scatter for rides with power+cadence, parameterized like the W′bal strip.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `activity-streams`: 2 MODIFIED requirements (persistence CHECK + retrieval shape gain `cadence`) + 2 ADDED (quadrant endpoint, MCP tool).
- `garmin-bridge`: 1 ADDED requirement — defensive cadence extraction into the stream post.
- `coach-dashboard`: 1 ADDED requirement — the workout-detail quadrant scatter.

## Impact

- **Code:** migration (CHECK widen); `internal/activitystreams/` ingest/retrieval + pure `quadrant.go`; bridge `_extract_streams` + tests; `apps/web` scatter.
- **API/MCP:** one GET, one read tool, golden regen additive, `task swag`; bridge deploy needed before cadence flows.
- **Operational:** cadence exists only for rides synced (or stream-re-posted) after the bridge update — the quadrant endpoint's `cadence_stream_missing` covers history.
- **Out of scope (deferred):** run cadence (spm) uses, cadence-based metrics beyond the quadrant view, torque-effectiveness/pedal-smoothness (Garmin-computed, different data), crank length in athlete-config.
