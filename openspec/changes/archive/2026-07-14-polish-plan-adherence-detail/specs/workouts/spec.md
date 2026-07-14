## ADDED Requirements

### Requirement: The missed-session list cap is tunable per request

`GET /api/v1/workouts/adherence` SHALL accept an optional `missed_limit=` parameter bounded to
[1, 200] (default 50, preserving existing behavior when omitted) controlling the maximum
`missed_sessions` returned; `missed_sessions_truncated` SHALL reflect truncation against the
effective limit. An out-of-bounds or non-integer value SHALL return `400 missed_limit_invalid`.
The `workout_adherence` MCP tool SHALL forward the optional arg.

#### Scenario: A raised limit returns the longer list

- **WHEN** 80 overdue-unfulfilled sessions exist and `missed_limit=100` is supplied
- **THEN** all 80 are returned oldest-first and `missed_sessions_truncated` is false

#### Scenario: Omission preserves today's behavior

- **WHEN** the parameter is omitted with 80 missed sessions
- **THEN** 50 are returned and `missed_sessions_truncated` is true

#### Scenario: An out-of-bounds limit is rejected

- **WHEN** `missed_limit=0` or `missed_limit=500` is supplied
- **THEN** the response is `400` with `missed_limit_invalid`

### Requirement: The weekly adherence trend supports opt-in zero-fill

`GET /api/v1/workouts/adherence` SHALL accept an optional `zero_fill=true` parameter causing the
`weekly` trend to emit every week in span — every Monday-started calendar week of the window, or
every plan-week ordinal in plan mode — with empty weeks carrying zeroed counts and a null
adherence rate (a week with nothing due has no rate). Omitted or `false` SHALL preserve the
existing sparse trend; a non-boolean value SHALL return `400 zero_fill_invalid`. Zero-fill SHALL
NOT alter the window totals or any populated bucket's values. The `workout_adherence` MCP tool
SHALL forward the optional arg.

#### Scenario: Zero-filled calendar trend is continuous

- **WHEN** the window spans 8 weeks of which 3 have classified sessions and `zero_fill=true`
- **THEN** the trend carries 8 buckets, the 5 empty ones with zeroed counts and null rate

#### Scenario: Plan mode fills every ordinal

- **WHEN** `plan_id` is supplied with `zero_fill=true` and the plan has 12 weeks
- **THEN** the trend carries 12 ordinal buckets, empty ones zeroed

#### Scenario: Populated buckets are unchanged by zero-fill

- **WHEN** the same request runs with and without `zero_fill=true`
- **THEN** every bucket present in both responses is value-identical
