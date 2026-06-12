# workouts — delta for add-garmin-scheduling

## ADDED Requirements

### Requirement: Workouts track Garmin scheduling identifiers

The system SHALL add two nullable columns to `workouts`: `garmin_workout_id`
(the id of the structured workout created in the Garmin library) and
`garmin_schedule_id` (the id of the calendar entry that schedules it). Both are
opaque Garmin identifiers — stored and echoed, never parsed. They are populated
when a planned workout is pushed to the watch and cleared when it is
unscheduled, enabling clean unschedule and re-push without double-creating in the
Garmin library.

#### Scenario: Columns exist after migration

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` has `garmin_workout_id` (TEXT NULL) and `garmin_schedule_id` (TEXT NULL)

#### Scenario: Ids are set on push and cleared on unschedule

- **WHEN** a planned workout is pushed to the watch and later unscheduled
- **THEN** both ids are populated by the push
- **AND** both ids are null after the unschedule
