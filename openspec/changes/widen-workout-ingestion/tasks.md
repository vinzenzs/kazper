# widen-workout-ingestion — tasks

## 1. Schema migration

- [x] 1.1 Verify the next free migration slot (expected `019`; check `ls internal/store/migrations/ | tail` for out-of-band slot-taking), then `task migrate:new NAME=widen_workout_ingestion`
- [x] 1.2 Up migration: `ALTER TABLE workouts ADD COLUMN` × 5 — `distance_m NUMERIC(10,1) NULL CHECK (distance_m IS NULL OR distance_m > 0)`, `avg_power_w INTEGER NULL CHECK (avg_power_w IS NULL OR avg_power_w > 0)`, `temperature_c NUMERIC(4,1) NULL CHECK (temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60))`, `sweat_loss_ml NUMERIC(10,1) NULL CHECK (sweat_loss_ml IS NULL OR sweat_loss_ml > 0)`, `session_group TEXT NULL`; plus `CREATE INDEX workouts_session_group_idx ON workouts (session_group) WHERE session_group IS NOT NULL`
- [x] 1.3 Down migration: drop the index and the five columns
- [x] 1.4 Migration round-trip test passes (existing harness applies up+down against testcontainers Postgres); existing rows read NULL for all five columns

## 2. workouts package — types, repo, service

- [x] 2.1 `types.go`: add `DistanceM *float64`, `AvgPowerW *int`, `TemperatureC *float64`, `SweatLossML *float64`, `SessionGroup *string` to `Workout` with `omitempty` JSON tags
- [x] 2.2 `repo.go`: extend `selectCols`, INSERT/UPSERT column lists (full-replace covers the five fields), the Patch SET-builder (value + explicit-NULL clear per field), and `List` with an optional `sessionGroup` filter predicate
- [x] 2.3 `service.go`: add sentinel errors `ErrDistanceMInvalid`, `ErrAvgPowerWInvalid`, `ErrTemperatureCInvalid`, `ErrSweatLossMLInvalid`, `ErrSessionGroupInvalid`; validation helpers (positivity, temperature range constants `TemperatureCMin/Max = -40/60`, trimmed non-empty ≤ 255 for session_group); extend `CreateInput` (5 value pointers) and `PatchInput` (5 value pointers + 5 `Clear*` bools); validate in Create and Patch paths
- [x] 2.4 Repo tests: upsert round-trip with all five fields, partial presence, NULL when omitted, UPSERT full-replace nulls an omitted field, DB CHECK enforcement (negative distance, temperature 100), patch set + clear per field, List filtered by session_group

## 3. workouts handlers

- [x] 3.1 `createRequest` + `buildCreateInput`: accept the five fields; map validation failures to `400 distance_m_invalid | avg_power_w_invalid | temperature_c_invalid | sweat_loss_ml_invalid | session_group_invalid` (temperature error carries `range: {min: -40, max: 60}`); `avg_power_w` non-integer rejection mirrors the `avg_hr` pattern
- [x] 3.2 PATCH handler: add the five fields to the `mutable` allowlist map; tri-state decode via `json.RawMessage` (`"null"` → `Clear*`, value → pointer) following the rpe precedent; wire new sentinel errors into the error-code mapping
- [x] 3.3 List handler: accept optional `?session_group=` and pass it through (window stays required; empty param = no filter)
- [x] 3.4 Bulk path: confirm the per-item `createRequest` decode picks up the new fields with per-item error codes (no bulk-specific code expected)
- [x] 3.5 Response boundary: `distance_m`, `sweat_loss_ml`, `temperature_c` rounded via `numfmt.Round1` wherever the existing numeric fields are rounded
- [x] 3.6 Handler tests: POST happy path with all five fields; each invalid-value error code; brick pair shares session_group; PATCH set/clear/leave-unchanged per field; PATCH validation matches POST; list `?session_group=` returns only matching legs in started_at order; missing window with session_group still `window_required`; swag annotations updated on all touched handlers

## 4. workoutfueling echo

- [x] 4.1 Add `sweat_loss_ml` + `temperature_c` (omitempty) to the top-level fueling response struct, populated from the workout row; do NOT echo `distance_m` / `avg_power_w` / `session_group`
- [x] 4.2 Tests: fueling response carries both when set, omits when NULL, never contains the three non-echoed keys (assert with `assert.NotContains`)

## 5. MCP server

- [x] 5.1 `tools_workouts.go`: `LogWorkoutArgs` + `PatchWorkoutArgs` gain the five optional fields with unit-explicit jsonschema descriptions (metres / watts / °C / ml; session_group = same key on every leg of a brick, e.g. Garmin parent activity id); patch arg forwarding preserves explicit JSON null for clears (rpe precedent)
- [x] 5.2 `ListWorkoutsArgs` gains optional `session_group`, forwarded as a query parameter
- [x] 5.3 Tool descriptions for `log_workout` / `patch_workout` updated (one sentence each: what the fields are, tri-state on patch); `list_workouts` description mentions the brick-legs filter
- [x] 5.4 MCP wrapper tests: explicit values forwarded verbatim, absent values omitted from the body, null-clear forwarded on patch, session_group query param forwarded; integration-test expected-tools list confirmed unchanged

## 6. Docs + verification

- [x] 6.1 `task swag` — regenerate `docs/` (required after the handler/request/response changes)
- [x] 6.2 README workouts subsection: one paragraph on the ingestion metrics + a brick example (two POSTs sharing `session_group`, then `GET /workouts?…&session_group=`)
- [x] 6.3 RUN_LOCAL: extend the fueling-rehearsal example with `sweat_loss_ml`/`temperature_c` in the workout body and note the fueling response now echoes them
- [x] 6.4 `task vet` + full `task test` green
- [x] 6.5 Out-of-repo coordination note (no code here): `garmin.py` push path should map Garmin distance/avgPower/temperature/estimated sweat loss and set `session_group` from multisport parent activity ids — tracked outside this repo
