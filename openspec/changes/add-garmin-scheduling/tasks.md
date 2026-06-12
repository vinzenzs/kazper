# Tasks: add-garmin-scheduling

## 1. Migration

- [x] 1.1 Verify migration head is `031` (after add-training-plan), then `task migrate:new NAME=add_workout_garmin_schedule_ids` (expect `032`)
- [x] 1.2 Up: `ALTER workouts ADD garmin_workout_id TEXT NULL, ADD garmin_schedule_id TEXT NULL`. Down: drop both columns

## 2. workouts additions

- [x] 2.1 `internal/workouts/types.go`: add `GarminWorkoutID *string`, `GarminScheduleID *string` (json omitempty)
- [x] 2.2 `repo.go`: a method to set/clear the two Garmin ids on a workout row; ensure list/get serialize them

## 3. Bridge write endpoints (`apps/garmin-bridge`, Python)

- [x] 3.1 Step-model → garminconnect payload builder: intents, end conditions (time/distance/lap-button/open), targets (HR/power zone, pace, RPE, absolute HR/power), repeat groups → `repeatGroupDTO`; unit tests on the translation
- [x] 3.2 `POST /workouts` → build payload, create in Garmin library, return `{garmin_workout_id}`
- [x] 3.3 `POST /schedule` → schedule workout id on a date, return `{garmin_schedule_id}`; `DELETE /schedule` → remove (no-op success if already gone)
- [x] 3.4 `GET /calendar?from&to` → list scheduled items
- [x] 3.5 Bridge README documents the four write/read endpoints

## 4. garmin-control orchestration (Go)

- [x] 4.1 `POST /garmin/schedule/workout {workout_id}`: load planned workout + template steps, call bridge create+schedule, store ids; if already scheduled, unschedule first (idempotent re-push); validation errors for non-planned / no-template; `503 garmin_disabled` when `GARMIN_BRIDGE_URL` unset
- [x] 4.2 `DELETE /garmin/schedule/workout/{workout_id}`: require stored `garmin_schedule_id`, call bridge delete, clear both ids; no-op success when unscheduled
- [x] 4.3 `POST /garmin/schedule/plan {plan_id, scope}`: resolve planned workouts in scope (week or date range), loop the single-workout path, collect per-item results (partial success); reuse the training-plan scope resolution
- [x] 4.4 `GET /garmin/calendar?from&to`: read-through to the bridge verbatim
- [x] 4.5 Wire endpoints in `internal/httpserver`; require auth (any identity); swag annotations

## 5. MCP tools

- [x] 5.1 `internal/mcpserver`: `garmin_schedule_workout`, `garmin_unschedule_workout`, `garmin_schedule_plan`, `garmin_list_scheduled`; one HTTP call each, verbatim; write tools auto-derive idempotency key; 503 surfaces as `isError=true`
- [x] 5.2 Bump expected-tools list in `mcp_integration_test.go`

## 6. Tests

- [x] 6.1 garmin-control handler tests against a stub bridge: push stores ids; re-push unschedules then reschedules; unschedule clears ids; non-planned rejected; plan-scope partial success; 503 when bridge URL unset
- [x] 6.2 Bridge payload-builder unit tests cover warmup/interval/repeat/cooldown and each target kind

## 7. Docs + verification

- [x] 7.1 `task swag`; README REST table gains the schedule endpoints; README MCP table gains the four tools; note the push-plan-to-watch flow
- [x] 7.2 `task vet` + `task test` green; `openspec validate add-garmin-scheduling --strict` passes
