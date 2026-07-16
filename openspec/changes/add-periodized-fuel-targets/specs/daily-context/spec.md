## ADDED Requirements

### Requirement: The daily context carries today's and tomorrow's fuel-plan classification

The `/api/v1/context/daily` payload SHALL include a compact `fuel_plan` block — for today and
tomorrow: the tier, g/kg, `suggested_carbs_g` (when weight data exists), and `plan_missing`
where applicable — positioned beside the goals data so the morning check-in reads the
classification without an extra call. When fuel-plan data cannot be computed at all the block
SHALL be omitted, never an error, and the rest of the payload SHALL be unaffected.

#### Scenario: The check-in sees today and tomorrow

- **WHEN** today is an easy day and tomorrow holds a heavy planned session
- **THEN** `/context/daily` carries both classifications with their suggested carb targets

#### Scenario: Absent plan data omits the block

- **WHEN** no fuel-plan classification is computable
- **THEN** the payload has no `fuel_plan` key and is otherwise unchanged
