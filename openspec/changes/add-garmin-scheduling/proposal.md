# Proposal: add-garmin-scheduling

## Why

This is the last plane of `garmin.py` and the only genuinely new *direction*:
writing planned sessions **to** the Garmin watch. `garmin.py`'s `schedule`,
`unschedule`, `create-workouts`, `schedule-week`, and `schedule-yoga` commands
upload structured workouts to the Garmin library and place them on the calendar,
so the watch guides the athlete through warmups, intervals, and target zones.
With the plan now a system of record (`add-training-plan`) and sessions defined
as structured templates (`add-workout-templates`), the backend can compile a
planned workout into a real Garmin workout and schedule it — closing the loop:
the plan lives in the API, flows out to the watch, and the completed activity
flows back via the read-sync bridge.

## What Changes

- **Bridge gains write endpoints** (extends `apps/garmin-bridge`): `POST /workouts`
  (build a structured Garmin workout from our template step model and create it
  in the Garmin library → returns a Garmin workout id), `POST /schedule` (place a
  Garmin workout on a date → returns a Garmin schedule id), `DELETE /schedule`
  (remove a scheduled item), and `GET /calendar` (read scheduled items for a date
  range). **The bridge owns the translation** from our clean step model to
  garminconnect's `executableStepDTO`/`repeatGroupDTO` payload — that shape never
  enters the backend.
- **Backend scheduling proxy + orchestration** (extends the `garmin-control`
  package): `POST /garmin/schedule/workout` (push one planned workout — compile
  via the bridge, schedule it, store the returned Garmin ids on the workout row),
  `DELETE /garmin/schedule/workout/{id}` (unschedule using the stored id),
  `POST /garmin/schedule/plan` (push every planned workout in a plan-week or date
  range — loops the single-workout path), and `GET /garmin/calendar` (read
  through to the bridge). All return `503 garmin_disabled` when
  `GARMIN_BRIDGE_URL` is unset.
- **Modified `workouts` capability** (additive): two new nullable columns,
  `garmin_workout_id` and `garmin_schedule_id`, tracking what has been pushed so
  unschedule and re-push are clean and the same session is never double-created
  in the Garmin library.
- **MCP tools**: `garmin_schedule_workout`, `garmin_unschedule_workout`,
  `garmin_schedule_plan`, `garmin_list_scheduled` — one HTTP call each, body
  forwarded verbatim, write tools auto-deriving an idempotency key.
- **Migration `032_add_workout_garmin_schedule_ids`** — `ALTER workouts ADD
  garmin_workout_id, garmin_schedule_id`.
- **Docs**: `task swag`; README REST + MCP tables; MCP expected-tools list bump;
  bridge README documents the write endpoints.

## Capabilities

### New Capabilities

<!-- None. garmin-bridge and garmin-control already exist (add-garmin-bridge /
     add-garmin-mcp-login). This change adds requirements to them. -->

### Modified Capabilities

- `garmin-bridge`: gains workout-create / schedule / unschedule / calendar-read
  endpoints and the step-model → Garmin-payload translation.
- `garmin-control`: gains backend scheduling proxy + orchestration endpoints that
  push planned workouts to the watch and track the returned Garmin ids.
- `workouts`: gains `garmin_workout_id` and `garmin_schedule_id` tracking columns.
- `mcp-server`: gains the Garmin scheduling tools.

## Impact

- **Depends on** `add-workout-templates` (steps to compile),
  `add-training-plan` (the planned workouts to push), `add-garmin-bridge` (the
  bridge to extend), and `add-garmin-mcp-login` (the `garmin-control` package and
  `GARMIN_BRIDGE_URL`). Lands after all four.
- **New code**: bridge write handlers + Garmin payload builder (Python),
  `garmin-control` scheduling endpoints + orchestration (Go), `workouts` column
  additions, MCP tool group, migration `032`.
- **Security**: scheduling acts on the athlete's calendar; endpoints require
  authentication (any first-party identity). No credentials transit this path —
  the bridge holds them, as in the read-sync and login flows.
- **No breaking changes**: all additive.
