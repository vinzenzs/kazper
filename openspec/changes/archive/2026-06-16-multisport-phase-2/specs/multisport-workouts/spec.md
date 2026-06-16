## ADDED Requirements

### Requirement: A multisport template can be scheduled through a training-plan slot

The system SHALL allow a training-plan slot to reference a multisport template
(via `plan_slots.multisport_template_id`), so a triathlon/brick can sit on a plan
day like any single-sport session. When such a slot is materialized, the plan
SHALL emit a `multisport` planned workout (per the training-plan and workouts
requirements), and that planned workout SHALL reach the watch through the same
effective-program → Garmin push path as single-sport planned workouts — compiling
to one multi-segment Garmin workout via the bridge's multisport payload. This is
in addition to the existing direct compile-and-schedule action; both paths
produce a single multisport Garmin workout from the same template. Deleting a
multisport template referenced by any plan slot SHALL be prevented (the slot
reference is `ON DELETE RESTRICT`).

#### Scenario: A planned multisport slot pushes through the plan path

- **WHEN** a plan slot references a multisport template, the plan is materialized,
  and the resulting planned workout is pushed to Garmin
- **THEN** the bridge receives the multisport form (segments in order) and creates
  one multisport Garmin workout placed on the planned date

#### Scenario: A referenced multisport template cannot be deleted

- **WHEN** a delete is attempted on a multisport template that a plan slot
  references
- **THEN** the delete is rejected (ON DELETE RESTRICT) and the template persists

#### Scenario: The direct schedule action still works independently

- **WHEN** a multisport template is scheduled via the direct
  `POST /garmin/schedule/multisport` action (no plan slot)
- **THEN** it compiles and schedules as in Phase 1, unaffected by the plan path
