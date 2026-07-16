## ADDED Requirements

### Requirement: Workouts carry an optional indoor/outdoor environment

The system SHALL support a nullable `environment` field on workouts constrained to
`indoor` | `outdoor` (CHECK-enforced; null = not stated). It SHALL be settable on
`POST /workouts` and the bulk upsert, tri-state on PATCH (a value sets, the empty string
clears, an omitted key leaves unchanged — the established sentinel convention), serialized
`omitempty`, and validated to `400 environment_invalid` on any other value. The field answers
"does ambient weather apply to this session" — downstream heat logic SHALL treat null as
assumed-outdoor and say so, never silently.

#### Scenario: Setting and clearing via PATCH tri-state

- **WHEN** a workout is PATCHed with `{"environment":"indoor"}`, then later with
  `{"environment":""}`
- **THEN** the field is stored as `indoor`, then cleared to null; a PATCH omitting the key
  leaves it unchanged

#### Scenario: An invalid value is rejected

- **WHEN** a write carries `{"environment":"garage"}`
- **THEN** the response is `400` with `environment_invalid`
