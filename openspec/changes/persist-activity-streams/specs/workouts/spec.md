# workouts Specification (delta)

## ADDED Requirements

### Requirement: Workouts carry stream-derived execution-metric columns

The system SHALL add three nullable columns to `workouts`, written exclusively by the
`activity-streams` ingest/recompute path (never accepted on `POST /workouts`,
`POST /workouts/bulk`, or `PATCH /workouts/{id}` — they are not part of the mutable
field set):

- `variability_index` (NUMERIC(4, 2) NULL, CHECK `variability_index IS NULL OR variability_index > 0`)
- `efficiency_factor` (NUMERIC(6, 3) NULL, CHECK `efficiency_factor IS NULL OR efficiency_factor > 0`)
- `decoupling_pct` (NUMERIC(5, 1) NULL, CHECK `decoupling_pct IS NULL OR (decoupling_pct BETWEEN -100 AND 100)`)

Every existing row SHALL carry NULL for all three (no back-fill — historical workouts
have no stored streams until re-synced). Read paths (`GET /workouts`,
`GET /workouts/{id}`) SHALL echo the three fields under the standard omitempty rule:
present when set, keys absent when NULL. The values are unit-isolated performance
signals and SHALL feed no nutrition, hydration, or energy total.

#### Scenario: Columns are added nullable with no back-fill

- **WHEN** the migration adding the three execution-metric columns is applied to a
  database with existing `workouts` rows
- **THEN** every existing row carries NULL for `variability_index`,
  `efficiency_factor`, and `decoupling_pct`
- **AND** the migration succeeds without back-filling any of them

#### Scenario: GET echoes execution metrics when set

- **WHEN** a workout's streams have been ingested and its metrics derived
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body includes `variability_index`, `efficiency_factor`, and
  `decoupling_pct` with their derived values

#### Scenario: GET omits execution metrics when NULL

- **WHEN** a workout has no derived execution metrics
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body omits all three keys (omitempty)

#### Scenario: Execution metrics are not client-writable

- **WHEN** the client sends `variability_index`, `efficiency_factor`, or
  `decoupling_pct` in a `POST /workouts` or `PATCH /workouts/{id}` body
- **THEN** the fields are not part of the accepted request shape and are never written
  from client input (the only writers are the stream ingest and recompute paths)
