## ADDED Requirements

### Requirement: Materialize adopts an existing completed activity for a slot

Materialize SHALL attempt **reverse reconciliation** for a plan slot that has no
workout row yet (per the workouts capability): it SHALL adopt exactly one
matching unlinked completed activity (same sport, within ±1 local day of the
slot's date, same-day preferred) by linking it to the slot instead of creating a
duplicate planned row. When no such activity matches, the
system SHALL create the slot's planned workout exactly as before. Re-materializing
a slot that already has a workout row SHALL follow the existing
`plan_slot_id`-keyed, `status='planned'`-guarded path and SHALL NOT create a
second row or re-adopt. This applies to single-sport slots; a `multisport` slot
has no completed `multisport` activity to adopt (completed bricks are decomposed
into single-sport rows) and SHALL always materialize its planned row.

#### Scenario: A slot with a prior import adopts it instead of duplicating

- **WHEN** a completed `garmin` workout of a sport was imported before its plan
  was materialized, and the matching slot is then materialized
- **THEN** that completed workout is linked to the slot (it gains the slot's
  `plan_slot_id` and `template_id`) and no separate planned row is created

#### Scenario: A slot with no matching import materializes normally

- **WHEN** a slot is materialized and no adoptable completed activity matches
- **THEN** a `planned` workout row is created for the slot as before

#### Scenario: Re-materialize after adoption is idempotent

- **WHEN** a slot whose activity was already adopted is materialized again
- **THEN** no duplicate row is created and the completed activity is unchanged
