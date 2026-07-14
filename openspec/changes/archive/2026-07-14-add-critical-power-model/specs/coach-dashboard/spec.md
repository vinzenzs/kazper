## ADDED Requirements

### Requirement: The stats route renders a critical-power panel

The dashboard's `/stats` route SHALL render a critical-power panel fed by
`GET /api/v1/workouts/cp-model`: a readout of `cp_watts`, `w_prime_kj`, and fit quality, plus the
in-band effort points with the fitted hyperbola `P(t) = CP + W′/t` on a log-x duration axis
(matching the power-curve panel's conventions), with a window selector consistent with the other
stats panels. When the endpoint returns a null model the panel SHALL render a degraded state
naming the reason (e.g. not enough long efforts in the window) rather than an empty or broken
chart. The panel MAY juxtapose the configured FTP from the athlete-config data the dashboard
already fetches; it SHALL NOT imply the estimate has been applied anywhere.

#### Scenario: A fitted model renders readout and curve

- **WHEN** `/stats` loads and the cp-model endpoint returns a fitted model
- **THEN** the panel shows CP, W′, and fit quality, and plots the contributing points with the
  fitted curve

#### Scenario: A null model renders the degraded state

- **WHEN** the cp-model endpoint returns `model: null` with a reason
- **THEN** the panel renders a readable explanation instead of a chart and the rest of `/stats`
  is unaffected
