## MODIFIED Requirements

### Requirement: A training plan is stored as plan → weeks → slots

The system SHALL persist a training plan across three tables: `training_plans`
(name, optional `race_id`, `start_date` = the Monday of week 1, optional notes),
`plan_weeks` (an `ordinal >= 1` unique within the plan, an optional `phase_id`,
notes), and `plan_slots` (a `weekday` 0–6 where 0=Monday, an `ordinal` ordering
sessions within a day, a template reference, and an optional `time_of_day`). A
slot's template reference SHALL be **either** a single-sport `template_id`
(referencing `workout_templates`) **or** a `multisport_template_id` (referencing
`multisport_templates`), with **exactly one** of the two set; a slot with neither
or both SHALL be rejected (a DB CHECK enforces the exclusivity as a backstop).
Weeks cascade-delete with their plan and slots with their week. Both template
references SHALL be `ON DELETE RESTRICT` so a referenced template cannot be
deleted while a slot points at it.

#### Scenario: Tables are created with the documented shape

- **WHEN** the migration set is applied to a clean database
- **THEN** `training_plans`, `plan_weeks`, and `plan_slots` exist with the
  documented columns and foreign keys
- **AND** `plan_weeks` has a UNIQUE constraint on `(plan_id, ordinal)`
- **AND** `plan_weeks.ordinal` and `plan_slots.weekday` carry CHECK constraints
  (`ordinal >= 1`, `weekday BETWEEN 0 AND 6`)
- **AND** `plan_slots.template_id` references `workout_templates(id)` ON DELETE RESTRICT
  and `plan_slots.multisport_template_id` references `multisport_templates(id)` ON DELETE RESTRICT
- **AND** a CHECK constraint enforces exactly one of `template_id` /
  `multisport_template_id` is non-null

#### Scenario: A slot references a multisport template

- **WHEN** a slot is created with a `multisport_template_id` and no `template_id`
- **THEN** the slot persists and is returned referencing that multisport template

#### Scenario: A slot with neither or both template references is rejected

- **WHEN** a slot is created with both `template_id` and `multisport_template_id`,
  or with neither
- **THEN** the response is a validation error and nothing is persisted

### Requirement: Materialize expands the plan into planned workouts idempotently

The system SHALL expose `POST /training-plans/{id}/materialize` accepting a scope
of a single week (`{scope:"week",week:N}`), a date range
(`{scope:"range",from,to}`), or the whole plan (`{scope:"all"}`). For each
in-scope slot it SHALL compute the date as
`plan.start_date + (week.ordinal-1) weeks + slot.weekday`, derive a time window
from `slot.time_of_day` (or a default stacked by `slot.ordinal`) and the
**session length** defined below, and UPSERT a `workouts` row with
`status='planned'` and `plan_slot_id`. For a **single-sport** slot the row
carries the slot's template's sport and name and `template_id`; for a
**multisport** slot the row carries `sport='multisport'`, the multisport
template's name, and `multisport_template_id` (and no `template_id`). The session
length SHALL be derived in order: (1) the sum of the slot's **effective program**
step durations when every step (across all segments, for a multisport slot) is
bounded by time; (2) otherwise the template's `estimated_duration_sec` (for a
multisport template, the sum of its segments' durations when all are bounded);
(3) otherwise a one-hour default — so a slot's `duration_overrides` move the
materialized window in lockstep with the watch workout. The upsert SHALL be keyed
on `plan_slot_id` so re-running updates the same rows, and its update SHALL apply
only where the existing row's `status` is `planned`. When a (week, weekday) has
more than one slot, the materialized workouts SHALL share a generated
`session_group`. The response SHALL return the planned workouts created or updated.

#### Scenario: A duration override moves the materialized session length

- **WHEN** a slot's effective program (template steps + `duration_overrides`) sums
  to 80min by time and the week is materialized
- **THEN** the planned workout's time window spans 80min, not the template's
  original `estimated_duration_sec`

#### Scenario: Materializing a week creates planned workouts on the right dates

- **WHEN** a plan with `start_date` = a Monday has a week 1 with a slot on weekday 2
  (Wednesday) and the client materializes week 1
- **THEN** a planned `workouts` row exists dated that Wednesday with the template's
  sport and name and `status='planned'`

#### Scenario: A multisport slot materializes a multisport planned workout

- **WHEN** a slot referencing a multisport template is materialized
- **THEN** a planned `workouts` row exists with `sport='multisport'`,
  `multisport_template_id` set, `template_id` null, and `status='planned'`

#### Scenario: Re-materializing is idempotent

- **WHEN** the same week is materialized twice
- **THEN** no duplicate planned workouts are created (the slot-keyed rows are
  updated in place)

#### Scenario: Materialize never reverts a fulfilled planned workout

- **WHEN** a planned workout has been marked `completed` (carrying its
  `plan_slot_id`) and its plan is re-materialized
- **THEN** the slot-keyed update is skipped for that row (guarded by
  `status='planned'`) and the completed workout and its actuals are unchanged

## ADDED Requirements

### Requirement: A multisport planned workout's effective program is a list of per-segment programs

When the planned workout is multisport (`sport='multisport'`), the system SHALL
resolve its **effective program** from the referenced multisport template as an
ordered list of **segments**, each carrying its own `sport` and its resolved step
program (or, for a transition segment, its duration). Each segment's steps SHALL
be run through the same effective-program target-resolution pass used for
single-sport workouts, keyed by **that segment's sport** — so a bike segment
resolves `power_zone` to absolute `power_w` and may carry a `secondary_target`,
while run/swim segments pass `power_zone` through (the per-segment-sport
resolution the single-sport engine deferred). Per-intent slot
`target_overrides`/`duration_overrides` SHALL NOT apply to a multisport slot;
supplying them on a multisport slot SHALL be a validation error. The effective
program SHALL be computed on read (template + athlete config), not snapshotted,
and SHALL be the single representation both `get_workout_program` and the Garmin
push path build from.

#### Scenario: A multisport effective program returns resolved per-segment programs

- **WHEN** a multisport planned workout's effective program is resolved and the
  athlete config defines power zones
- **THEN** the program is a list of segments in order, each with its sport and
  steps, and the bike segment's `power_zone` targets are resolved to `power_w`
  ranges while the run/swim segments' targets are unchanged

#### Scenario: Overrides on a multisport slot are rejected

- **WHEN** a multisport slot is created or patched with `target_overrides` or
  `duration_overrides`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: A transition segment surfaces in the effective program

- **WHEN** the multisport template contains a `transition` segment between two
  sport segments
- **THEN** the effective program lists that transition segment in order with its
  duration and no targets
