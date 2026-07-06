## ADDED Requirements

### Requirement: Per-activity best-effort records are computed from ingested streams and stored

The system SHALL expose `POST /api/v1/workouts/{id}/streams` accepting a workout's per-sample
time series (at least a **power** series in watts and/or a **speed** series in m/s, with sample
timestamps or a fixed cadence). For the referenced completed workout, the system SHALL compute
the **mean-maximal** value of each provided metric at a fixed set of standard durations (e.g. 5s,
15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) — the best rolling-window average of that length anywhere
in the activity — and SHALL store one best-effort record per (workout, metric, duration) in a
dedicated table keyed so a re-post **replaces** that workout's records rather than duplicating
them. A duration longer than the activity SHALL yield no record for that duration. The raw
streams SHALL NOT be required to persist beyond the computation (retention of a downsampled
stream is out of scope for this requirement). Power (W) and pace/speed values live only in this
capability's shapes and SHALL feed no nutrition, hydration, or energy total. An unknown workout
id SHALL return `404`; a workout with no usable series SHALL be accepted with no records written.

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
