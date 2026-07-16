## Why

"When this athlete runs faster, does the speed come from turnover or from stride length — and which one plateaus?" is a limiter question the coach currently cannot answer with math, only with vibes. Speed = cadence × step length, so the decomposition is fully determined by two streams — and Kazper already persists speed at 1 Hz for every synced run. The missing half is deliberate debt: the cadence-quadrant change wired cadence end to end but extracted only `directBikeCadence`, explicitly deferring "run cadence (spm) uses" — so run cadence never reaches `workout_streams` today. This change closes that deferral and builds the per-workout cadence-vs-stride read on top, following the compute-on-read pattern of quadrant/W′bal/intervals.

## What Changes

- **garmin-bridge:** `_extract_streams` additionally extracts run cadence from `directDoubleCadence` (Garmin's both-feet series, already in steps/min) when `directBikeCadence` is absent, defensive like the existing columns; posted with the existing stream POST. No migration — the `cadence` stream type already exists.
- `activity-streams` cadence storage/retrieval wording becomes **sport-native units**: rpm for rides (unchanged), spm for runs. Storage semantics are untouched — samples are stored as posted.
- **New endpoint** `GET /api/v1/workouts/{id}/stride` — run workouts only. Computed on read from paired speed+cadence samples: per-sample step length `speed / (spm/60)` (meters per step), samples with non-positive speed or cadence excluded and counted (standing/dropout, the quadrant convention). Returns speed-binned mean cadence and step length, a cadence-vs-stride **contribution split** across the speed range (how much of speed gain comes from turnover vs stride, and where each plateaus — the limiter verdict is decomposed, never a bare label), summary stats, and a downsampled scatter; `summary_only` supported.
- New `stride_analysis` agent/MCP tool (read tier, always `summary_only=true` — the scatter is chart data). Expected-tools list in the MCP integration test bumped.
- Workout-detail dashboard gains a cadence-vs-stride view for runs with speed+cadence, parameterized like the quadrant scatter.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `activity-streams`: 1 MODIFIED requirement (cadence unit wording becomes sport-native: rpm rides / spm runs) + 2 ADDED (stride endpoint, MCP tool).
- `garmin-bridge`: 1 MODIFIED requirement — cadence extraction gains the `directDoubleCadence` run fallback.
- `coach-dashboard`: 1 ADDED requirement — the run workout-detail cadence-vs-stride view.

## Impact

- **Code:** `internal/activitystreams/` pure `stride.go` + handler + tests; `internal/agenttools/registry_activitystreams.go`; bridge `_extract_streams` + tests; `apps/web` view.
- **API/MCP:** one GET, one read tool, `task swag`, MCP expected-tools bump; bridge deploy needed before run cadence flows.
- **Operational:** run cadence exists only for runs synced (or stream-re-posted) after the bridge update — the endpoint's `cadence_stream_missing` sentinel covers history honestly.
- **Out of scope (deferred):** cross-workout stride trends over time (needs this per-workout read first), grade-adjusted step length (no elevation stream is stored), vertical oscillation / ground-contact time (different Garmin columns, different change), walking-pace exclusion tuning beyond the non-positive filter (a `min_speed_mps` param is a design decision, not a schema one).
