# Tasks — persist activity streams + execution metrics

## 1. Migration

- [x] 1.1 Verify the current highest migration number first (`ls internal/store/migrations/ | tail` — head is around `054`; out-of-band work may have taken the next slot), then scaffold with `task migrate:new NAME=add_workout_streams`.
- [x] 1.2 Up migration: create `workout_streams` (`id` UUID PK, `workout_id` UUID NOT NULL REFERENCES `workouts(id)` ON DELETE CASCADE, `stream_type` TEXT CHECK IN `('power','speed','heart_rate')`, `samples` REAL[] NOT NULL, `sample_rate_hz` INTEGER NOT NULL DEFAULT 1, `sample_count` INTEGER NOT NULL, `created_at`/`updated_at`, UNIQUE `(workout_id, stream_type)`), plus `ALTER TABLE workouts ADD` `variability_index NUMERIC(4,2)`, `efficiency_factor NUMERIC(6,3)`, `decoupling_pct NUMERIC(5,1)` — all NULL, with the CHECK constraints from the workouts delta, no back-fill.
- [x] 1.3 Down migration: drop `workout_streams` and the three `workouts` columns.

## 2. New `internal/activitystreams/` package (standard capability shape)

- [x] 2.1 `types.go`: `StreamPayload` (`power`, `speed`, `heart_rate` `[]float64`, omitempty), `IngestResult` (`records_written`, `streams_stored`), `StreamsResponse` (`workout_id`, `sample_rate_hz`, `duration_s`, optional `downsample`, `streams` map with omitted absent types), `RecomputeResult`.
- [x] 2.2 `repo.go`: upsert-replace per (workout_id, stream_type) against `store.Querier`, load-all-for-workout, delete relies on FK cascade; store `sample_count` explicitly.
- [x] 2.3 `service.go`: ingest (persist streams → delegate best-effort replace to the `effortanalytics` service → derive/store execution metrics), retrieval with bucket-mean downsampling (bounds `[10, 5000]`), recompute (load stored streams → same derivations). Sentinel errors: `ErrWorkoutNotFound`, `ErrStreamsNotFound`, `ErrDownsampleInvalid` mapping 1:1 to `workout_not_found` / `streams_not_found` / `downsample_invalid`.
- [x] 2.4 Execution-metric computation: NP (30 s rolling avg, 4th-power mean, ¼ root, ≥ 20 min gate), VI = NP/mean power, EF = NP/mean HR (else mean speed/mean HR), decoupling = (r1−r2)/r1×100 over stream halves; HR zeros excluded from all HR means, < 80% valid-HR coverage → NULL HR-derived metrics; rounding VI 2 dp / EF 3 dp / decoupling 1 dp at the boundary. Unit-test the math directly (steady ride, run without power, dropout-heavy HR, < 20 min activity, negative decoupling).
- [x] 2.5 `handlers.go` with swag annotations: `POST /workouts/:id/streams` (moved route — contract preserved, response gains `streams_stored`), `GET /workouts/:id/streams`, `POST /workouts/:id/streams/recompute`; `Register(rg)`.

## 3. Effort-analytics + workouts adjustments

- [x] 3.1 Remove the `POST /workouts/:id/streams` route registration from `internal/effortanalytics/handlers.go` (power-curve stays); expose the best-effort computation for reuse by `activitystreams` (keep `IngestStreams` or extract a compute-and-replace entry point taking the resolved workout). Update the package doc comment (streams are now persisted by activity-streams).
- [x] 3.2 `internal/workouts/types.go` + repo scan lists: add `variability_index`, `efficiency_factor`, `decoupling_pct` (pointer fields, omitempty) to the Workout shape and read paths; confirm POST/bulk/PATCH request shapes do NOT accept them.
- [x] 3.3 `internal/httpserver/server.go`: instantiate `activitystreams` repo/service/handlers, inject the workouts repo and effortanalytics service, register routes.

## 4. Garmin bridge: heart-rate stream

- [x] 4.1 `apps/garmin-bridge/garmin_bridge/mapping.py` `_extract_streams`: also pull the `directHeartRate` column as `heart_rate` (gaps → 0, wholly non-positive series dropped); `map_workout_streams` docstring update.
- [x] 4.2 Bridge tests: extend `tests/test_mapping.py` (HR extraction, HR-only activity, no-HR activity unchanged) and `tests/test_effort_streams.py` / `tests/test_sync.py` fixtures so posted payloads carry `heart_rate` where the fixture detail has it.

## 5. MCP

- [x] 5.1 Add `recompute_workout_streams` to `internal/agenttools/` (new `registry_activitystreams.go`): args `{workout_id}`, one `POST /workouts/{id}/streams/recompute`, write tier with derived idempotency semantics matching the other write tools; description notes when to use it (threshold/logic changes) and that raw streams are deliberately not exposed.
- [x] 5.2 Regenerate the announced-schema golden (`go test -tags=goldengen ./internal/mcpserver/`) and run the MCP integration test (`-tags=integration`) — the announced-tools assertion derives from the registry, the golden file must include the new tool.

## 6. Tests, docs, verification

- [x] 6.1 Handler integration tests in `internal/activitystreams/` (testcontainers): ingest persists + replaces + still writes best-efforts + writes metrics; legacy two-series post; empty payload no-op; GET full-resolution + downsample + bounds errors + `streams_not_found`; recompute happy path + 404s; workout delete cascades stream rows; assert nutrition shapes never gain stream units (no power/speed/HR keys in summary responses).
- [x] 6.2 Update/extend `internal/workouts` tests: GET echoes the three metrics when set, omits when NULL; POST/PATCH cannot write them.
- [x] 6.3 Run `task swag` (moved + new handlers change the OpenAPI surface).
- [x] 6.4 Run `task vet` and `task test` (re-run flaky testcontainers packages with `-p 1` if boot contention appears); run bridge tests (`pytest` in `apps/garmin-bridge`).
