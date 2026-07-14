# effort-analytics Specification

## Purpose

Compute and serve per-activity mean-maximal best-effort records — power/speed at a fixed duration ladder — ingested from Garmin streams, plus the windowed power/pace curve aggregated from them. Raw streams are computed over in-request and not persisted; power and speed are unit-isolated and feed no nutrition, hydration, or energy total.
## Requirements
### Requirement: Per-activity best-effort records are computed from ingested streams and stored

The system SHALL expose `POST /api/v1/workouts/{id}/streams` accepting a workout's per-sample
time series (at least a **power** series in watts and/or a **speed** series in m/s, with sample
timestamps or a fixed cadence; an optional **heart_rate** series in bpm may accompany them for
the `activity-streams` capability). For the referenced completed workout, the system SHALL compute
the **mean-maximal** value of each provided power/speed metric at a fixed set of standard durations
(e.g. 5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) — the best rolling-window average of that length
anywhere in the activity — and SHALL store one best-effort record per (workout, metric, duration)
in a dedicated table keyed so a re-post **replaces** that workout's records rather than duplicating
them. A duration longer than the activity SHALL yield no record for that duration. The raw
streams ARE persisted by the `activity-streams` capability, and the system SHALL support
re-deriving a workout's best-effort records from those stored streams via its recompute path,
producing the same records the original ingest would (heart-rate series feed no best-effort
record — the mean-maximal ladder remains power/speed only). Power (W) and pace/speed values
live only in this capability's shapes and SHALL feed no nutrition, hydration, or energy total.
An unknown workout id SHALL return `404`; a workout with no usable series SHALL be accepted with
no records written.

#### Scenario: Posting a power stream computes and stores best efforts

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a power series for a completed workout
- **THEN** the system stores that workout's best mean power at each standard duration up to the
  activity length

#### Scenario: Re-posting replaces, does not duplicate

- **WHEN** the same workout's streams are posted a second time
- **THEN** its best-effort records are replaced, not duplicated

#### Scenario: Durations longer than the activity are skipped

- **WHEN** the activity is shorter than a standard duration
- **THEN** no best-effort record is written for that duration

#### Scenario: A workout without a usable series writes nothing

- **WHEN** the posted streams contain no power or speed series
- **THEN** the request is accepted and no best-effort records are written

#### Scenario: Recompute from stored streams reproduces the ladder

- **WHEN** a workout's best-effort records are re-derived from its persisted streams via the
  `activity-streams` recompute path
- **THEN** the resulting (workout, metric, duration) records replace the prior set and match
  what ingesting the same series would produce

#### Scenario: A heart-rate series yields no best-effort record

- **WHEN** the posted streams include a `heart_rate` series
- **THEN** no best-effort record with a heart-rate metric is written (the ladder stays
  power/speed only)

### Requirement: The aggregated power/pace curve is readable over a window

The system SHALL expose `GET /api/v1/workouts/power-curve?from=&to=&sport=&tz=` returning, for
each standard duration, the **best** best-effort value achieved across completed workouts in the
range (mean-maximal curve), together with the workout id and date that value came from. The
metric SHALL be selectable/derivable by sport (power for bike, pace for run/swim). The range
SHALL support at least a full year. Invalid or missing range/`tz` parameters SHALL return `400`
with the same error contract as the other range endpoints (`range_required`, `date_invalid`,
`range_invalid`, `range_too_large` with `max_days`, `tz_invalid`). An empty window SHALL return
an empty curve rather than erroring.

#### Scenario: Curve returns the windowed mean-maximal values

- **WHEN** `GET /api/v1/workouts/power-curve?from=&to=&sport=bike` is requested and best-effort
  records exist in the range
- **THEN** the response returns, per standard duration, the best power achieved and the
  contributing workout id and date

#### Scenario: Empty window returns an empty curve

- **WHEN** no best-effort records exist in the requested range
- **THEN** the endpoint returns an empty curve without erroring

#### Scenario: Invalid range is rejected

- **WHEN** the range/`tz` params are missing or invalid
- **THEN** the endpoint returns `400` with the shared range error contract

### Requirement: The power curve is readable over MCP

The system SHALL expose a `power_curve` MCP tool that issues a single
`GET /api/v1/workouts/power-curve` and forwards the response verbatim, so the coaching agent can
reason about mean-maximal power/pace without client-side computation.

#### Scenario: Agent reads the curve in one call

- **WHEN** the agent invokes `power_curve` with `from`, `to`, `sport`, and optional `tz`
- **THEN** the tool issues one `GET /workouts/power-curve` request and returns the curve as the
  tool result

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

