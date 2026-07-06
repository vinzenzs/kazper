## 1. Backend: storage + effort-analytics package

- [x] 1.1 Add migration `053_add_workout_best_efforts` (`workout_id` FK, `metric`, `duration_s`, `value`, `achieved_at`; unique `(workout_id, metric, duration_s)` for replace-on-repost). Verify 053 is the next free number before committing.
- [x] 1.2 Create `internal/effortanalytics/types.go`: stream ingest payload (power/speed sample arrays + timestamps/cadence), best-effort record, curve response shapes
- [x] 1.3 Implement the mean-maximal algorithm in `service.go` (rolling-window best mean at the fixed duration ladder 5s…60m; skip durations longer than the activity; handle gaps/variable cadence)
- [x] 1.4 Create `repo.go`: upsert-replace a workout's best-effort records; windowed `MAX ... GROUP BY duration_s` curve query with sport filter
- [x] 1.5 Create `handlers.go`: `POST /workouts/{id}/streams` (404 unknown id, accept-with-no-records when no usable series) and `GET /workouts/power-curve` (shared range/tz error contract); swag annotations
- [x] 1.6 Wire the package + routes in `internal/httpserver/server.go`
- [x] 1.7 Go tests: algorithm fixtures (constant / spike / gaps), re-post replaces, durations-longer-than-activity skipped, curve aggregation, error contract
- [x] 1.8 Run `task swag`

## 2. MCP mirror

- [x] 2.1 Add `internal/agenttools/registry_effortanalytics.go` with a `power_curve` tool issuing the single `GET /workouts/power-curve`
- [x] 2.2 Register the tool group and bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go`

## 3. Garmin bridge: stream ingestion

- [x] 3.1 Add a `get_activity_details` fetch in `garmin_client.py`, individually guarded like the existing per-activity detail fetches
- [x] 3.2 Extract per-sample power (W) and speed (m/s) series in `mapping.py`, defensively omitting on unexpected/absent shape
- [x] 3.3 In `sync.py`, after the workout upsert, POST the extracted streams to `POST /workouts/{id}/streams` using the upserted workout id; guard so failure/absence omits streams without aborting the day
- [x] 3.4 Python tests: activity with power stream posts, activity without stream skips, re-run re-posts idempotently

## 4. Frontend: power/pace curve

- [x] 4.1 Add curve response types in `api/types.ts` and a `usePowerCurve(from, to, sport)` hook in `api/hooks.ts`
- [x] 4.2 Build a visx log-x curve chart (best value vs duration) in the analyst idiom
- [x] 4.3 Add sport (power/pace) + window selectors on the stats surface and route the chart in
- [x] 4.4 (Optional) overlay the per-workout curve on the Phase-1 detail page — **DEFERRED**: the downsampled-stream open question resolved to "default off" (streams are discarded after computing best-efforts), so there is no per-workout curve data to overlay. Revisit only if raw-stream retention is added later.
- [x] 4.5 Vitest: curve renders, selector re-query, empty-state

## 5. Verify & (optional) backfill

- [x] 5.1 Run `task test`, `task vet`, and the bridge Python test suite; build the SPA and confirm `webembed` tests pass
- [x] 5.2 (Optional follow-on) re-drive recent history through the bridge backfill path to populate curves for existing activities — **DEFERRED**: operational task, run against live Garmin when the curve is actually wanted (needs real credentials + rate-limit care; not part of the code change).
