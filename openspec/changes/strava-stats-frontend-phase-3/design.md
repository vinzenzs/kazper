## Context

Phases 1–2 surfaced existing data and added volume totals — both pure read-side, no ingestion. Phase 3 is different: the power/pace curve needs **per-second time-series streams** that the backend does not store today. The bridge (`apps/garmin-bridge/`) already fetches per-activity detail (HR-zone time via `get_activity_hr_in_timezones`, per-lap `get_activity_splits`, `get_activity_exercise_sets`, weather) and maps it into the `/workouts` payload; each fetch is individually guarded so one failure doesn't sink the day. `garminconnect` (0.3.6, already on `main`) additionally exposes `get_activity_details(activity_id)`, which returns the sampled metric arrays (power, speed, HR, altitude, cadence, distance) — the raw material for a mean-maximal curve. Migration head is `052`.

The "power curve" (a.k.a. mean-maximal power, MMP) is: for each duration D, the highest average power over any D-length window in the activity; the windowed curve is the per-duration max across all activities in a date range. It is exactly a best-effort aggregation.

## Goals / Non-Goals

**Goals:**
- Ingest per-activity power/speed streams from Garmin without destabilizing the guarded daily sync.
- Compute and store compact per-activity best-effort (mean-maximal) records.
- Serve an aggregated windowed power/pace curve to the frontend and the coach agent.

**Non-Goals:**
- No long-term raw-stream storage (see the core decision).
- No critical-power / W′ modeling, HR curve, or FTP auto-estimation (the curve is their input, proposed separately if wanted).
- No segments/leaderboards, no live streaming.
- No merging of power/pace into nutrition/energy totals (unit isolation).
- No new visual language — analyst idiom only.

## Decisions

### Core: store precomputed best-efforts, not raw streams
`POST /workouts/{id}/streams` computes the mean-maximal value at a fixed duration ladder (5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) in-request and stores one row per (workout, metric, duration) in `workout_best_efforts`. Raw streams are discarded after computation.

- **Why:** the curve *is* the mean-maximal aggregation — best-efforts serve it directly. A single-user Postgres shouldn't accumulate ~10k–20k samples × several metrics per activity forever when the derived product is a few dozen rows. Best-efforts also make the windowed curve a trivial `MAX ... GROUP BY duration` query.
- **Trade-off:** future analytics needing raw samples (e.g. a different smoothing, W′balance) require re-pulling from Garmin. Acceptable — the bridge's backfill path already re-drives history.
- **Alternative — store raw streams (a `workout_streams` blob/table):** rejected as the default for storage weight and because nothing on the roadmap needs raw samples. Left as a reversible future option (add a table; the ingest endpoint already receives the arrays).

### Compute in the backend (Go), bridge stays a mapper
The bridge extracts and forwards the sample arrays; the backend owns the mean-maximal algorithm (a testable rolling-window max-mean in `internal/effortanalytics/service.go`).

- **Why:** matches the repo's "bridge maps Garmin → REST, backend owns logic" split. The algorithm is deterministic and unit-testable in Go (fixture stream → expected ladder), where the rest of the domain logic and its test harness live. The wire cost (one activity's 1 Hz arrays) is bounded and transient.
- **Alternative — compute on the bridge, post the compact ladder:** less wire data, but puts non-trivial math in the thin Python mapper and splits the algorithm from its natural test home. Rejected unless payload size proves a problem.

### Ingest endpoint is per-workout, separate from the workout upsert
Streams post to `POST /workouts/{id}/streams` after the activity has been upserted to `/workouts` (which mints the id / dedups by `external_id`). The bridge already has the workout id from the upsert response.

- **Why:** keeps the (large, optional, individually-guarded) stream payload off the main activity upsert, so a stream failure never risks the workout row. Mirrors how the sync already treats each detail fetch as independent.

### Curve endpoint mirrors the range/tz idiom
`GET /workouts/power-curve?from=&to=&sport=&tz=` reuses the shared range param + tz + range-cap + error contract (like Phase 2's `/workouts/summary` and `/summary/range`), with a year-plus cap so all-time-ish windows work.

### MCP + frontend
`power_curve` MCP tool (single GET) per 1:1 convention. Frontend: `usePowerCurve(from,to,sport)` + a visx log-x line chart on the stats surface; optional per-workout curve overlay on the Phase-1 detail page.

## Risks / Trade-offs

- **[Sync latency/volume — one extra detail fetch + POST per activity]** → The detail call is the heaviest Garmin fetch. Mitigate by guarding it like the others (failure omits streams, day continues) and only fetching for activities likely to have power/speed (or attempt-and-omit). Backfilling history is a separate, throttled run.
- **[Discarding raw streams is a one-way door per-activity]** → If a new raw-sample analytic is wanted later, history must be re-pulled from Garmin. Accepted; documented. Optional escape hatch (open question): retain a downsampled 1 Hz stream for the detail chart only.
- **[Mean-maximal correctness]** → Off-by-one/edge bugs in the rolling window (gaps, variable sample cadence, zero-padding) produce subtly wrong curves. Mitigate with Go unit tests over known fixtures (constant power, single spike, gaps) asserting the exact ladder.
- **[Pace vs power unification]** → Run/swim have no power; the curve uses pace (best speed → pace) for those sports. Keep metric selection explicit by sport to avoid a meaningless "run power" curve.
- **[Garmin detail schema drift]** → `get_activity_details` metric descriptors are positional/keyed arrays that can change. Isolate parsing in the bridge mapping layer with defensive extraction (omit on unexpected shape), consistent with existing `_map_splits`/weather handling.

## Migration Plan

1. **Backend first (ingest + storage + curve), behind no flag but unused until the bridge posts:**
   - Migration `053_add_workout_best_efforts` (`workout_id` FK, `metric`, `duration_s`, `value`, `achieved_at`; unique on (workout_id, metric, duration_s) for replace-on-repost).
   - `internal/effortanalytics/` (types/repo/service/handlers/tests): mean-maximal algorithm; `POST /workouts/{id}/streams`; `GET /workouts/power-curve`. Wire in `httpserver.Run()`. `task swag`.
   - MCP `power_curve` + bump `mcp_integration_test.go` expected-tools.
2. **Bridge:** `get_activity_details` fetch in `garmin_client.py`; extract power/speed in `mapping.py`; POST in `sync.py`; guard + idempotency; Python tests.
3. **Frontend:** `usePowerCurve` + types; visx log-x curve chart + sport/window selectors; optional detail-page overlay; vitest.
4. **Backfill (optional follow-on):** re-drive recent history through the bridge to populate curves for existing activities.
- **Rollback:** revert frontend + bridge; the backend table/endpoints are inert without posts. Dropping the migration is clean (no other table references it).

## Open Questions

- **Retain a downsampled stream for the detail chart?** Default: no (best-efforts only). If the per-workout detail page wants a power-over-time trace, retain a 1 Hz (or coarser) stream per activity in a separate table — decide during apply based on whether the detail overlay is in scope.
- **Duration ladder** — the exact set (and whether it's configurable) — fix the default ladder above; revisit if the chart wants finer resolution.
- **Compute site** — reaffirm backend-computes unless the 1 Hz payload proves too heavy over the bridge→backend hop in practice.
- **Backfill scope** — how far back to populate curves on first ship (all history vs last N months), balancing Garmin rate limits.
