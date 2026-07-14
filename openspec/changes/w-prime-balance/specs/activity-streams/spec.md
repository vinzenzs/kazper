## ADDED Requirements

### Requirement: W′ balance is computed on read from a stored power stream

The system SHALL expose `GET /api/v1/workouts/{id}/w-prime-balance?cp_watts=&w_prime_kj=`
computing the W′ balance series over the workout's stored 1 Hz power stream with the
differential (Froncioni–Clarke–Skiba) model: starting from the supplied `w_prime_kj`, each
sample above `cp_watts` depletes the balance by `(P − CP)` joules per second and each sample
below it recharges by `(W′ − bal)·(CP − P)/W′` per second. Both parameters are REQUIRED and
SHALL be validated as positive numbers (`400 cp_invalid` / `400 w_prime_invalid`); the response
SHALL echo them back. The response SHALL carry `duration_s`, a `summary` with `min_w_prime_kj`,
`min_at_s`, `end_w_prime_kj`, `max_depletion_pct`, and `time_below_25_pct_s`, and the balance
`series`, which SHALL honor the existing stream `downsample` convention (bucket-mean, bounds
[10, 5000], echoed when applied, full resolution when omitted, `400 downsample_invalid` when out
of bounds); `summary_only=true` SHALL omit the series. The balance SHALL NOT be clamped at zero —
a negative minimum indicates the supplied parameters understate demonstrated capacity and
`max_depletion_pct` MAY exceed 100. Kilojoule and percentage values SHALL be rounded to 1 decimal
at the response boundary only. The computation SHALL persist nothing and SHALL NOT read
`athlete-config`. An unknown workout id SHALL return `404 workout_not_found`; a workout with no
stored streams SHALL return `404 streams_not_found`; stored streams without a power series SHALL
return `404 power_stream_missing`.

#### Scenario: A power-streamed workout returns summary and series

- **WHEN** `GET /workouts/{id}/w-prime-balance?cp_watts=250&w_prime_kj=20` targets a workout
  with a stored power stream
- **THEN** the response is `200` with the echoed params, `duration_s`, the summary fields, and a
  full-resolution balance series

#### Scenario: Constant supra-CP power depletes linearly

- **WHEN** the stored power stream holds a constant `P > cp_watts` for `t` seconds
- **THEN** the series decreases by `(P − cp_watts)·t` joules over that span and
  `summary.min_w_prime_kj` reflects the final depleted value

#### Scenario: The balance goes negative rather than clamping

- **WHEN** the workout's supra-CP work exceeds the supplied `w_prime_kj`
- **THEN** `summary.min_w_prime_kj` is negative and `summary.max_depletion_pct` exceeds 100

#### Scenario: summary_only omits the series

- **WHEN** the request includes `summary_only=true`
- **THEN** the response carries params, `duration_s`, and `summary` but no `series`

#### Scenario: Missing or non-positive parameters are rejected

- **WHEN** `cp_watts` or `w_prime_kj` is absent, non-numeric, or ≤ 0
- **THEN** the response is `400` with `cp_invalid` or `w_prime_invalid` respectively

#### Scenario: Stored streams without power are distinguished from no streams

- **WHEN** the workout has stored streams but none of type `power`
- **THEN** the response is `404` with `{"error":"power_stream_missing"}`

### Requirement: The W′ balance summary is readable over MCP

The system SHALL expose a `w_prime_balance` MCP tool (read tier) that issues a single
`GET /api/v1/workouts/{id}/w-prime-balance` with `summary_only=true` always applied, forwarding
the response body verbatim — the agent receives the params echo and summary but never the series
(raw per-sample data remains chart data, not a reasoning input). The tool SHALL accept
`workout_id`, `cp_watts`, and `w_prime_kj`, and its description SHALL point at the `cp_model`
tool as the parameter source and state that the computation is advisory.

#### Scenario: Agent reads a workout's W′bal summary in one call

- **WHEN** the agent invokes `w_prime_balance` with a workout id and CP/W′ values
- **THEN** the tool issues one GET with `summary_only=true` and returns the summary body
  verbatim, with no series field present
