# activity-streams Specification

## Purpose
TBD - created by archiving change persist-activity-streams. Update Purpose after archive.
## Requirements
### Requirement: Raw activity streams are persisted per workout

The system SHALL persist each ingested per-workout sample stream in a `workout_streams`
table: one row per (workout, stream type) holding the complete contiguous 1 Hz sample
series as a native array column. The table SHALL carry `id` (UUID PRIMARY KEY),
`workout_id` (UUID NOT NULL REFERENCES `workouts(id)` ON DELETE CASCADE), `stream_type`
(TEXT NOT NULL, CHECK IN `('power','speed','heart_rate')`), `samples` (REAL[] NOT NULL),
`sample_rate_hz` (INTEGER NOT NULL DEFAULT 1), `sample_count` (INTEGER NOT NULL), and
audit timestamps, with a UNIQUE constraint on `(workout_id, stream_type)` so a re-post
REPLACES that workout's stored series rather than duplicating it. Samples SHALL be
stored faithfully as posted (gap-filled zeros included); interpretation of zeros happens
at compute time, not at storage time. Stream values are unit-isolated — power (W), speed
(m/s), and heart rate (bpm) SHALL feed no nutrition, hydration, or energy total.

#### Scenario: Posted streams are stored one row per stream type

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a body with `power`, `speed`,
  and `heart_rate` arrays for an existing workout
- **THEN** three `workout_streams` rows exist for that workout, one per stream type,
  each holding the full sample array and its `sample_count`

#### Scenario: Re-posting replaces the stored series

- **WHEN** the same workout's streams are posted a second time with a different `power`
  array
- **THEN** exactly one `power` row exists for that workout, holding the new array

#### Scenario: Deleting the workout deletes its streams

- **WHEN** a workout with stored streams is deleted via `DELETE /workouts/{id}`
- **THEN** its `workout_streams` rows are removed by the cascade

### Requirement: Stream ingest persists the raw arrays alongside the derived records

The existing `POST /api/v1/workouts/{id}/streams` endpoint SHALL, in addition to
computing best-effort records (per the `effort-analytics` capability), persist every
posted series (`power`, `speed`, and the new optional `heart_rate` array in bpm) to
`workout_streams` and derive the workout's execution metrics (see the execution-metrics
requirement). The endpoint's path, method, auth, and error contract SHALL be preserved:
an unknown workout id returns `404 workout_not_found`; a malformed body returns
`400 invalid_body`; an empty payload is accepted and writes nothing (no stream rows, no
best-effort rows, execution metrics untouched). The response SHALL keep
`records_written` and additionally report `streams_stored` (the count of stream rows
written) — an additive, non-breaking change to the response shape.

#### Scenario: Ingest persists and derives in one call

- **WHEN** the bridge posts `{"power":[...],"speed":[...],"heart_rate":[...]}` for a
  completed workout
- **THEN** the streams are persisted, the best-effort ladder is replaced, and the
  workout's execution-metric columns are written
- **AND** the response carries `records_written` and `streams_stored: 3`

#### Scenario: A two-series legacy post remains valid

- **WHEN** an older bridge posts only `power` and `speed` (no `heart_rate`)
- **THEN** the request succeeds exactly as before, storing two stream rows
- **AND** HR-derived execution metrics are left NULL

#### Scenario: An empty payload writes nothing

- **WHEN** the posted body contains no series
- **THEN** the request is accepted, no stream rows or best-effort records are written,
  and any previously stored streams and metrics are left untouched

### Requirement: Stored streams are retrievable with optional downsampling

The system SHALL expose `GET /api/v1/workouts/{id}/streams` returning the workout's
stored series as
`{workout_id, sample_rate_hz, duration_s, streams: {power, speed, heart_rate}}` with
absent stream types omitted. An optional `downsample=<points>` query parameter, bounded
to `[10, 5000]`, SHALL reduce each series by equal-width bucket means to at most that
many points and echo the applied `downsample` in the response; when omitted, full
resolution is returned. An unknown workout id SHALL return `404 workout_not_found`; a
workout with no stored streams SHALL return `404 streams_not_found`; an out-of-bounds
or non-integer `downsample` SHALL return `400 downsample_invalid` with the range.

#### Scenario: Full-resolution retrieval

- **WHEN** the client calls `GET /workouts/{id}/streams` for a workout with a stored
  10,800-sample power series
- **THEN** the response's `streams.power` carries all 10,800 samples and `duration_s`
  reflects the series length at `sample_rate_hz`

#### Scenario: Downsampled retrieval for graphs

- **WHEN** the client calls `GET /workouts/{id}/streams?downsample=500`
- **THEN** each returned series has at most 500 points, each the mean of its bucket
- **AND** the response echoes `downsample: 500`

#### Scenario: No stored streams returns 404

- **WHEN** the workout exists but has no `workout_streams` rows
- **THEN** the endpoint returns `404` with `{"error":"streams_not_found"}`

#### Scenario: Out-of-bounds downsample is rejected

- **WHEN** the client passes `downsample=5` or `downsample=99999` or `downsample=abc`
- **THEN** the endpoint returns `400` with
  `{"error":"downsample_invalid","range":{"min":10,"max":5000}}`

### Requirement: Best-efforts and execution metrics are recomputable from stored streams

The system SHALL expose `POST /api/v1/workouts/{id}/streams/recompute` (no body) that
loads the workout's stored streams, replaces its best-effort records using the same
computation as ingest, and rewrites its execution-metric columns — enabling re-analysis
when derivation logic or athlete thresholds change, without re-syncing from Garmin. An
unknown workout id SHALL return `404 workout_not_found`; a workout with no stored
streams SHALL return `404 streams_not_found`. The response SHALL report
`records_written` like the ingest endpoint.

#### Scenario: Recompute re-derives from storage

- **WHEN** `POST /workouts/{id}/streams/recompute` is called for a workout with stored
  streams
- **THEN** its best-effort ladder is replaced and its execution-metric columns are
  rewritten from the stored samples
- **AND** the stored streams themselves are unchanged

#### Scenario: Recompute without stored streams returns 404

- **WHEN** recompute is called for a workout that has no stored streams
- **THEN** the endpoint returns `404` with `{"error":"streams_not_found"}`
- **AND** the workout's existing best-effort records are untouched

### Requirement: Execution metrics are derived from streams at ingest and recompute

The system SHALL derive three execution-quality metrics from the streams and store them
on the workout row (columns specified in the `workouts` capability), at both ingest and
recompute:

- **Variability Index** = NP / mean power, where NP is the fourth-root of the mean of
  the fourth powers of the 30-second rolling average of the power series; computed only
  from a power stream spanning at least 20 minutes, else NULL.
- **Efficiency Factor** = NP / mean HR when a qualifying power stream exists, else
  mean speed / mean HR when a speed stream exists; NULL without valid HR.
- **Aerobic decoupling** = `(r1 − r2) / r1 × 100` where `r1`/`r2` are the
  output-per-heartbeat ratios (power preferred, else speed, each over mean valid HR) of
  the first and second halves of the stream; requires at least 20 minutes and valid HR
  coverage in both halves, else NULL.

HR samples equal to zero SHALL be treated as sensor dropout and excluded from all HR
means; when valid HR samples cover less than 80% of the stream, every HR-derived metric
SHALL be NULL rather than computed. Power and speed zeros SHALL remain in their means
(coasting is signal). Metrics SHALL be rounded at the response boundary (VI 2 dp,
EF 3 dp, decoupling 1 dp).

#### Scenario: A steady ride with power and HR yields all three metrics

- **WHEN** a 90-minute power + heart-rate stream with full HR coverage is ingested
- **THEN** the workout row carries `variability_index`, `efficiency_factor`, and
  `decoupling_pct` derived per the formulas above

#### Scenario: A run without power uses speed for EF and decoupling

- **WHEN** a 60-minute speed + heart-rate stream (no power) is ingested
- **THEN** `efficiency_factor` is mean speed / mean HR and `decoupling_pct` uses the
  speed:HR ratio halves
- **AND** `variability_index` is NULL (no power stream)

#### Scenario: Poor HR coverage yields NULL HR-derived metrics

- **WHEN** the heart-rate series has valid (non-zero) samples covering less than 80% of
  the stream
- **THEN** `efficiency_factor` and `decoupling_pct` are NULL
- **AND** `variability_index` is still computed when a qualifying power stream exists

#### Scenario: A short activity yields no NP-based metrics

- **WHEN** the posted power stream spans less than 20 minutes
- **THEN** `variability_index` and power-based `efficiency_factor` and `decoupling_pct`
  are NULL

### Requirement: MCP exposes recompute but deliberately not raw stream retrieval

The system SHALL expose a `recompute_workout_streams` MCP tool that issues a single
`POST /api/v1/workouts/{id}/streams/recompute` and forwards the response verbatim, so
the coaching agent can trigger re-derivation after threshold or logic changes. The raw
stream ingest and retrieval endpoints SHALL NOT be mirrored as MCP tools — a deliberate,
documented exception to the REST↔MCP 1:1 convention: the ingest POST is a bridge-only
write path (it has never had a tool), and a raw 10k+-sample array response is unusable
in an agent context, while every agent-relevant derivative (execution metrics on the
workout row, best-efforts via `power_curve`) is already MCP-reachable.

#### Scenario: Agent triggers a recompute in one call

- **WHEN** the agent invokes `recompute_workout_streams` with a workout id
- **THEN** the tool issues one `POST /workouts/{id}/streams/recompute` and returns the
  response as the tool result

#### Scenario: No raw-streams tools are announced

- **WHEN** the MCP server announces its tool list
- **THEN** it includes `recompute_workout_streams` but no tool wrapping
  `POST /workouts/{id}/streams` or `GET /workouts/{id}/streams`

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

