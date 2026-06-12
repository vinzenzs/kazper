# garmin-control — delta for add-garmin-scheduling

## ADDED Requirements

### Requirement: Pushing a planned workout compiles, schedules, and tracks Garmin ids

The system SHALL expose `POST /garmin/schedule/workout` accepting a `workout_id`
that refers to a planned workout (`status='planned'`) with a `template_id`. It
SHALL load the template's steps, call the bridge to create the structured Garmin
workout and schedule it on the workout's date, and persist the returned
`garmin_workout_id` and `garmin_schedule_id` onto the workout row. When the
workout already carries a `garmin_schedule_id`, the system SHALL unschedule the
prior entry before scheduling the new one, so a re-push leaves no orphan on the
calendar. The endpoint SHALL require authentication and SHALL return
`503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset.

#### Scenario: A planned workout is pushed to the watch

- **WHEN** an authenticated client `POST`s `/garmin/schedule/workout` with a
  planned workout's id
- **THEN** the backend compiles and schedules it via the bridge
- **AND** stores the returned `garmin_workout_id` and `garmin_schedule_id` on the workout
- **AND** returns the updated workout

#### Scenario: Re-pushing replaces the prior calendar entry

- **WHEN** a workout that already has a `garmin_schedule_id` is pushed again
- **THEN** the prior scheduled entry is unscheduled before the new one is created
- **AND** the workout's ids are updated to the new entry

#### Scenario: Pushing a non-planned workout is rejected

- **WHEN** the target workout is not `status='planned'` or has no `template_id`
- **THEN** the response is a validation error and nothing is scheduled

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Unscheduling a workout clears its Garmin link

The system SHALL expose `DELETE /garmin/schedule/workout/{workout_id}` that
requires a stored `garmin_schedule_id`, calls the bridge to remove the scheduled
entry, and clears both `garmin_schedule_id` and `garmin_workout_id` on the
workout row. It SHALL require authentication and return `503 garmin_disabled`
when the bridge URL is unset.

#### Scenario: Unschedule removes the entry and clears the ids

- **WHEN** an authenticated client `DELETE`s `/garmin/schedule/workout/{id}` for a
  scheduled workout
- **THEN** the bridge removes the calendar entry
- **AND** the workout's `garmin_schedule_id` and `garmin_workout_id` are cleared

#### Scenario: Unscheduling an unscheduled workout is a no-op success

- **WHEN** the workout has no `garmin_schedule_id`
- **THEN** the response indicates nothing was scheduled, without error

### Requirement: Pushing a plan scope schedules every planned workout in it

The system SHALL expose `POST /garmin/schedule/plan` accepting a plan scope (a
plan-week or a date range, mirroring materialize) and SHALL push each planned
workout in that scope through the single-workout path. Per-workout failures SHALL
be collected and returned rather than aborting the batch. It SHALL require
authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: A plan week is scheduled in one call

- **WHEN** an authenticated client `POST`s `/garmin/schedule/plan` for a plan week
  containing several planned workouts
- **THEN** each planned workout in that week is scheduled on the watch
- **AND** the response reports per-workout results

#### Scenario: One bad item does not abort the batch

- **WHEN** one workout in the scope fails to compile or schedule
- **THEN** the others are still scheduled
- **AND** the response reports the failure alongside the successes

### Requirement: The backend reads the Garmin calendar through the bridge

The system SHALL expose `GET /garmin/calendar` accepting a date range and
returning the bridge's calendar response verbatim, for reconciliation. It SHALL
require authentication and return `503 garmin_disabled` when the bridge URL is
unset.

#### Scenario: Calendar read passes through

- **WHEN** an authenticated client `GET`s `/garmin/calendar` with a date range
- **THEN** the backend forwards to the bridge's `GET /calendar` and returns its response verbatim
