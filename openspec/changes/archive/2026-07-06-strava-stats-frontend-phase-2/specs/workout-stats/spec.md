## ADDED Requirements

### Requirement: Workout volume is aggregated over a date range into per-day and windowed totals

The system SHALL expose `GET /api/v1/workouts/summary?from=&to=&tz=` returning, for completed
workouts in the range `[from, to]` (both inclusive, bucketed by calendar day in the requested
`tz`): a per-day series and a window total. Each per-day bucket and the window total SHALL carry
`count`, `total_duration_min`, `total_distance_m`, `total_elevation_gain_m`, `total_kcal`, and a
`by_sport` breakdown. Nullable workout metrics (`distance_m`, `elevation_gain_m`, `kcal_burned`)
SHALL be summed present-only — a workout missing a metric contributes nothing to that sum rather
than zeroing it — and numeric outputs SHALL be rounded at the response boundary via `numfmt`.
Only completed workouts SHALL be counted; planned workouts SHALL be excluded.

`from` and `to` are `YYYY-MM-DD`. Missing either SHALL return `400 range_required`; an
unparseable date SHALL return `400 date_invalid`; `from` after `to` SHALL return
`400 range_invalid`; an invalid `tz` SHALL return `400 tz_invalid`. The range SHALL support at
least a full year so year-to-date is a first-class window; a range exceeding the endpoint's day
cap SHALL return `400 range_too_large` with `max_days`.

Workout distance, elevation, and duration SHALL appear only in this response shape and SHALL NOT
be merged into any nutrition, hydration, or energy total. A multisport workout SHALL contribute
to `by_sport` as a single `multisport` entry (no per-leg distance attribution).

#### Scenario: Range totals and per-day series render

- **WHEN** `GET /api/v1/workouts/summary?from=2026-01-01&to=2026-01-31&tz=Europe/Berlin` is
  requested and completed workouts exist in that range
- **THEN** the response returns per-day buckets and a window total, each carrying `count`,
  `total_duration_min`, `total_distance_m`, `total_elevation_gain_m`, `total_kcal`, and
  `by_sport`

#### Scenario: Unmeasured metrics are summed present-only

- **WHEN** the range contains a workout with `distance_m` null
- **THEN** that workout adds to `count` and `total_duration_min` but contributes nothing to
  `total_distance_m` (the sum is not zeroed for the whole day)

#### Scenario: Planned workouts are excluded

- **WHEN** the range contains both completed and planned workouts
- **THEN** only the completed workouts are counted in the totals and per-day series

#### Scenario: Year-to-date is a supported range

- **WHEN** the requested range spans up to a full calendar year (≤ the endpoint day cap)
- **THEN** the request succeeds and returns per-day buckets across the whole range

#### Scenario: Invalid range is rejected

- **WHEN** `from` is after `to`, a date is unparseable, `from`/`to` is missing, the range
  exceeds the day cap, or `tz` is invalid
- **THEN** the endpoint returns `400` with `range_invalid`, `date_invalid`, `range_required`,
  `range_too_large` (with `max_days`), or `tz_invalid` respectively

### Requirement: Workout volume totals are readable over MCP

The system SHALL expose a `training_totals` MCP tool that issues a single
`GET /api/v1/workouts/summary` and forwards the response verbatim, so the coaching agent can
answer volume questions ("how far have I ridden this month") without client-side aggregation.

#### Scenario: Agent reads training volume in one call

- **WHEN** the agent invokes `training_totals` with a `from`, `to`, and optional `tz`
- **THEN** the tool issues one `GET /workouts/summary` request and returns the aggregated
  totals as the tool result
