# activity-streams Specification (delta)

## ADDED Requirements

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
