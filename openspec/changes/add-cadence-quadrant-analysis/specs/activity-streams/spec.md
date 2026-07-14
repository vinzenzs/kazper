## MODIFIED Requirements

### Requirement: Raw activity streams are persisted per workout

The system SHALL persist each ingested per-workout sample stream in a `workout_streams`
table: one row per (workout, stream type) holding the complete contiguous 1 Hz sample
series as a native array column. The table SHALL carry `id` (UUID PRIMARY KEY),
`workout_id` (UUID NOT NULL REFERENCES `workouts(id)` ON DELETE CASCADE), `stream_type`
(TEXT NOT NULL, CHECK IN `('power','speed','heart_rate','cadence')`), `samples` (REAL[] NOT
NULL), `sample_rate_hz` (INTEGER NOT NULL DEFAULT 1), `sample_count` (INTEGER NOT NULL), and
audit timestamps, with a UNIQUE constraint on `(workout_id, stream_type)` so a re-post
REPLACES that workout's stored series rather than duplicating it. Samples SHALL be
stored faithfully as posted (gap-filled zeros included); interpretation of zeros happens
at compute time, not at storage time. Stream values are unit-isolated — power (W), speed
(m/s), heart rate (bpm), and cadence (rpm) SHALL feed no nutrition, hydration, or energy
total; cadence SHALL additionally feed no best-effort record and no execution metric.

#### Scenario: Posted streams are stored one row per stream type

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a body with `power`, `speed`,
  and `heart_rate` arrays for an existing workout
- **THEN** three `workout_streams` rows exist for that workout, one per stream type,
  each holding the full sample array and its `sample_count`

#### Scenario: A cadence series is stored like any other stream

- **WHEN** the posted body additionally carries a `cadence` array in rpm
- **THEN** a fourth `workout_streams` row of type `cadence` holds it, and no best-effort or
  execution-metric record derives from it

#### Scenario: Re-posting replaces the stored series

- **WHEN** the same workout's streams are posted a second time with a different `power`
  array
- **THEN** exactly one `power` row exists for that workout, holding the new array

#### Scenario: Deleting the workout deletes its streams

- **WHEN** a workout with stored streams is deleted via `DELETE /workouts/{id}`
- **THEN** its `workout_streams` rows are removed by the cascade

### Requirement: Stored streams are retrievable with optional downsampling

The system SHALL expose `GET /api/v1/workouts/{id}/streams` returning the workout's
stored series as
`{workout_id, sample_rate_hz, duration_s, streams: {power, speed, heart_rate, cadence}}` with
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

#### Scenario: A stored cadence series is served alongside the others

- **WHEN** the workout has stored `power` and `cadence` rows
- **THEN** the response's `streams` object carries both series at equal length

#### Scenario: No stored streams returns 404

- **WHEN** the workout exists but has no `workout_streams` rows
- **THEN** the endpoint returns `404` with `{"error":"streams_not_found"}`

#### Scenario: Out-of-bounds downsample is rejected

- **WHEN** the client passes `downsample=5` or `downsample=99999` or `downsample=abc`
- **THEN** the endpoint returns `400` with
  `{"error":"downsample_invalid","range":{"min":10,"max":5000}}`

## ADDED Requirements

### Requirement: Quadrant analysis is computed on read from paired power and cadence streams

The system SHALL expose
`GET /api/v1/workouts/{id}/quadrant?cp_watts=&cadence_rpm=&crank_mm=` computing per-sample
circumferential pedal velocity (`CPV = cadence × crank × 2π / 60`) and average effective pedal
force (`AEPF = power / CPV`) over samples where both power and cadence are positive, classifying
each against the reference point implied by the REQUIRED `cp_watts` and `cadence_rpm`
(`400 cp_invalid` / `cadence_invalid` when absent or non-positive) and the OPTIONAL `crank_mm`
(default 172.5, `400 crank_invalid` outside [100, 220]). The response SHALL echo the params and
carry a summary — quadrant time shares (1 decimal), `pedaling_s`, `excluded_s` (samples with
non-positive power or cadence), and the reference values (2 decimals) — plus a scatter
downsampled to at most 1000 paired points; `summary_only=true` SHALL omit the scatter. Rounding
SHALL occur at the response boundary only; nothing SHALL be persisted. Sentinels:
`404 workout_not_found`, `404 streams_not_found`, `404 power_stream_missing`,
`404 cadence_stream_missing`.

#### Scenario: Paired streams return shares and scatter

- **WHEN** the workout has stored power and cadence and valid params are supplied
- **THEN** the response carries the four quadrant shares, pedaling/excluded seconds, reference
  values, and a scatter of at most 1000 points

#### Scenario: Coasting is excluded, not diluting

- **WHEN** a span of samples has cadence 0 while power is 0
- **THEN** those seconds appear in `excluded_s` and no quadrant share includes them

#### Scenario: Missing cadence is its own sentinel

- **WHEN** the workout has stored streams but none of type `cadence`
- **THEN** the response is `404` with `{"error":"cadence_stream_missing"}`

### Requirement: The quadrant summary is readable over MCP

The system SHALL expose a `quadrant_analysis` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/quadrant` with `summary_only=true` always applied, forwarding the
body verbatim — shares and references are reasoning data, the scatter stays chart data. Args:
`workout_id`, `cp_watts`, `cadence_rpm`, optional `crank_mm`; the description SHALL point at
`cp_model` as the CP source.

#### Scenario: Agent reads quadrant shares in one call

- **WHEN** the agent invokes `quadrant_analysis` with a workout id and params
- **THEN** the tool issues one GET with `summary_only=true` and returns the summary verbatim
  with no scatter
