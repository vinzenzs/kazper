## ADDED Requirements

### Requirement: Wellness fields correlate against a PMC metric over a window

The system SHALL expose `GET /api/v1/wellness/correlation?from=&to=&tz=&metric=` pairing each
wellness field's entries with the same-local-date PMC value (`metric`: `tsb` default, `ctl`, or
`ramp_rate`; anything else `400 metric_invalid`) and returning per-field Spearman rank
correlation as `{n, rho}` (rho 2 decimals, boundary rounding). A field with fewer than 14
same-day pairs SHALL return `{n, reason: "insufficient_pairs"}` and no rho. Days lacking either
side SHALL simply not pair — no interpolation or carry-forward. The endpoint SHALL be
compute-on-read, persist nothing, and use the PMC range/`tz` error contract.

#### Scenario: A well-logged field returns its correlation

- **WHEN** fatigue has 30 same-day pairs with TSB in the window
- **THEN** the response carries `fatigue: {n: 30, rho: <value>}`

#### Scenario: A sparse field is gated, not guessed

- **WHEN** motivation has 5 pairs in the window
- **THEN** the response carries `motivation: {n: 5, reason: "insufficient_pairs"}` with no rho

#### Scenario: An unknown metric is rejected

- **WHEN** `metric=vo2max` is supplied
- **THEN** the response is `400` with `metric_invalid`

### Requirement: Wellness correlation is readable over MCP

The system SHALL expose a `wellness_correlation` MCP tool (read tier) issuing a single
`GET /api/v1/wellness/correlation` and forwarding the body verbatim (`from`/`to` required,
`tz`/`metric` optional); the description SHALL state that results are associations, not causes.

#### Scenario: Agent reads the correlation in one call

- **WHEN** the agent invokes `wellness_correlation` for the last 90 days
- **THEN** the tool issues one GET and returns the per-field results verbatim
