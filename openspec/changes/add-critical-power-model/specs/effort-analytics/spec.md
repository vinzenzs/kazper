## ADDED Requirements

### Requirement: A windowed critical-power model is computed from stored best efforts

The system SHALL expose `GET /api/v1/workouts/cp-model?from=&to=&tz=` fitting the 2-parameter
critical power model to the window's best-effort records (bike **power** metric only): for each
standard ladder duration within the validity band of **2 to 30 minutes inclusive**, the windowed
best value (the same per-duration maximum the power curve serves) SHALL become one fit point, and
the system SHALL compute an ordinary-least-squares fit in work–time form (`work_j = cp·t + w_prime`),
returning `model` with `cp_watts`, `w_prime_kj`, `r_squared`, and `rmse_w`, together with the
`points` used (duration, watts, contributing workout id and date). Values SHALL be rounded only at
the response boundary (`cp_watts`/`rmse_w`/`w_prime_kj` to 1 decimal, `r_squared` to 3). The fit
SHALL be computed on read and persist nothing. The endpoint SHALL NOT read or write
`athlete-config` — the estimate is advisory and comparison against the configured FTP belongs to
the consumer. Durations outside the validity band (sprints, 60m) SHALL NOT enter the fit. The
range/`tz` contract SHALL match the power-curve endpoint (`range_required`, `date_invalid`,
`range_invalid`, `range_too_large` with `max_days`, `tz_invalid`; at least a full year supported).

#### Scenario: A window with sufficient efforts returns the fitted model

- **WHEN** `GET /api/v1/workouts/cp-model?from=&to=` covers best efforts at three or more in-band
  durations spanning at least a 3× duration ratio
- **THEN** the response is `200` with `model.cp_watts`, `model.w_prime_kj`, `model.r_squared`,
  `model.rmse_w`, and one point per in-band duration carrying its watts, workout id, and date

#### Scenario: Sprint and 60-minute efforts are excluded from the fit

- **WHEN** the window contains best efforts at 5s/15s/30s/1m/60m as well as in-band durations
- **THEN** only durations between 2 and 30 minutes inclusive appear in `points` and drive the fit

#### Scenario: Insufficient data degrades to a null model with a reason

- **WHEN** fewer than 3 in-band durations have a best effort in the window
- **THEN** the response is `200` with `model: null` and `reason: "insufficient_points"`, and any
  in-band points found are still returned

#### Scenario: A too-narrow duration span is refused a fit

- **WHEN** the in-band points exist but the longest duration is less than 3× the shortest
- **THEN** the response is `200` with `model: null` and `reason: "span_too_narrow"`

#### Scenario: Invalid range is rejected with the shared contract

- **WHEN** the range/`tz` params are missing or invalid
- **THEN** the endpoint returns `400` with the same error vocabulary as the power-curve endpoint

### Requirement: The critical-power model is readable over MCP

The system SHALL expose a `cp_model` MCP tool (read tier) that issues a single
`GET /api/v1/workouts/cp-model` and forwards the response body verbatim, so the coaching agent can
reason about the data-derived CP/W′ estimate — including comparing it against the configured FTP
it reads via the existing athlete-config tools — without client-side computation. The tool
description SHALL state that the estimate is advisory (CP approximates FTP) and that applying a
new threshold goes through the existing athlete-config update flow.

#### Scenario: Agent reads the model in one call

- **WHEN** the agent invokes `cp_model` with `from`, `to`, and optional `tz`
- **THEN** the tool issues one `GET /workouts/cp-model` request and returns the body verbatim as
  the tool result
