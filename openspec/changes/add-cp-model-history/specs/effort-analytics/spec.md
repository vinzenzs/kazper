## ADDED Requirements

### Requirement: The critical-power model is computable as a rolling-window history

The system SHALL expose `GET /api/v1/workouts/cp-model/history?from=&to=&tz=&window_days=`
running the existing CP2 fit at each Monday anchor within the range, each over the trailing
`window_days` window (default 90, bounds [30, 365] → `400 window_days_invalid`). Each anchor
SHALL carry its date and either the fitted model (`cp_watts`, `w_prime_kj`, `r_squared`,
`rmse_w` — the single-fit rounding rules) or `model: null` with the existing gate `reason`;
gated anchors SHALL remain in the series, never dropped. The range SHALL support at least a
full year with the power-curve error contract, and the endpoint SHALL be compute-on-read,
persist nothing, and read no `athlete-config` data.

#### Scenario: A season range returns a weekly fit series

- **WHEN** the endpoint is called over six months with default `window_days`
- **THEN** the response carries one anchor per Monday, each with a fitted model or a null model
  and gate reason

#### Scenario: A thin window stays in the series as a gap

- **WHEN** an anchor's trailing window has fewer than 3 in-band durations
- **THEN** that anchor appears with `model: null` and `reason: "insufficient_points"`

#### Scenario: Out-of-bounds window_days is rejected

- **WHEN** `window_days=10` or `window_days=999` is supplied
- **THEN** the response is `400` with `window_days_invalid`

### Requirement: The CP-model history is readable over MCP

The system SHALL expose a `cp_model_history` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/cp-model/history` and forwarding the body verbatim, with `from`/`to`,
optional `tz`/`window_days` args; the description SHALL note that configured-FTP comparison
composes with the athlete-config history tool.

#### Scenario: Agent reads the estimate's trend in one call

- **WHEN** the agent invokes `cp_model_history` over a season
- **THEN** the tool issues one GET and returns the anchor series verbatim
