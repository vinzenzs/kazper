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
(m/s), heart rate (bpm), and cadence (sport-native: rpm for rides, spm for runs) SHALL
feed no nutrition, hydration, or energy total; cadence SHALL additionally feed no
best-effort record and no execution metric.

#### Scenario: Posted streams are stored one row per stream type

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a body with `power`, `speed`,
  and `heart_rate` arrays for an existing workout
- **THEN** three `workout_streams` rows exist for that workout, one per stream type,
  each holding the full sample array and its `sample_count`

#### Scenario: A cadence series is stored like any other stream

- **WHEN** the posted body additionally carries a `cadence` array (rpm for a ride, spm
  for a run — stored as posted, sport disambiguates the unit)
- **THEN** a fourth `workout_streams` row of type `cadence` holds it, and no best-effort or
  execution-metric record derives from it

#### Scenario: Re-posting replaces the stored series

- **WHEN** the same workout's streams are posted a second time with a different `power`
  array
- **THEN** exactly one `power` row exists for that workout, holding the new array

#### Scenario: Deleting the workout deletes its streams

- **WHEN** a workout with stored streams is deleted via `DELETE /workouts/{id}`
- **THEN** its `workout_streams` rows are removed by the cascade

## ADDED Requirements

### Requirement: Stride analysis is computed on read from paired speed and cadence streams

The system SHALL expose `GET /api/v1/workouts/{id}/stride?min_speed_mps=&summary_only=`
for **run** workouts, computing per-sample step length (`step_length_m = speed / (spm / 60)`,
meters per single step) over samples where both speed and cadence are positive, and
bucketing qualifying samples into fixed-width 0.25 m/s speed bins over the observed range.
The response SHALL carry per-bin `seconds`, mean `cadence_spm` (1 decimal), and mean
`step_length_m` (2 decimals); a contribution split — the time-weighted least-squares slopes
of `ln(cadence)` and `ln(step_length)` against `ln(speed)` over the bin means, reported as
`cadence_contribution_pct` and `step_contribution_pct` (1 decimal, summing to 100) — and
summary counts `analyzed_s` and `excluded_s` (samples with non-positive speed or cadence).
When the qualifying bins span less than 0.5 m/s of speed range, the contribution split
SHALL be `null` with `reason: "insufficient_speed_range"` while the bins are still
returned. A scatter of at most 1000 systematically-thinned `(speed_mps, cadence_spm,
step_length_m)` points SHALL be included unless `summary_only=true`. The OPTIONAL
`min_speed_mps` (bounds [0.5, 5.0], `400 min_speed_invalid` outside them) SHALL exclude
samples below it (counted in `excluded_s`) and be echoed when applied. Rounding SHALL occur
at the response boundary only; nothing SHALL be persisted; step length and cadence SHALL
feed no nutrition, hydration, or energy total. Sentinels: `404 workout_not_found`,
`404 streams_not_found`, `404 speed_stream_missing`, `404 cadence_stream_missing`, and
`409 sport_unsupported` when the workout's sport is not run.

#### Scenario: A run with paired streams returns bins, split, and scatter

- **WHEN** the workout is a run with stored speed and cadence covering easy pace through
  intervals
- **THEN** the response carries speed bins with seconds, mean cadence, and mean step
  length, a cadence/step contribution split summing to 100, `analyzed_s`/`excluded_s`,
  and a scatter of at most 1000 points

#### Scenario: A steady-state run refuses to name a limiter

- **WHEN** the run's qualifying bins span less than 0.5 m/s
- **THEN** the bins are returned but the contribution split is `null` with
  `reason: "insufficient_speed_range"`

#### Scenario: Standing and dropouts are excluded, not diluting

- **WHEN** a span of samples has speed 0 or cadence 0
- **THEN** those seconds appear in `excluded_s` and no bin or fit includes them

#### Scenario: A walk-break cutoff is applied on request

- **WHEN** the client passes `min_speed_mps=1.8`
- **THEN** samples slower than 1.8 m/s are excluded and counted, and the response echoes
  the applied `min_speed_mps`

#### Scenario: A ride is rejected as the wrong sport

- **WHEN** the workout's sport is not run
- **THEN** the response is `409` with `{"error":"sport_unsupported"}`

#### Scenario: Missing cadence is its own sentinel

- **WHEN** the run has stored streams but none of type `cadence`
- **THEN** the response is `404` with `{"error":"cadence_stream_missing"}`

### Requirement: The stride summary is readable over MCP

The system SHALL expose a `stride_analysis` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/stride` with `summary_only=true` always applied, forwarding the
body verbatim — bins and the contribution split are reasoning data, the scatter stays chart
data. Args: `workout_id`, optional `min_speed_mps`; the description SHALL note the tool
applies to runs and reads best on runs with pace variety (intervals, fartlek, progressions).

#### Scenario: Agent reads the stride decomposition in one call

- **WHEN** the agent invokes `stride_analysis` with a run workout id
- **THEN** the tool issues one GET with `summary_only=true` and returns the bins, split,
  and counts verbatim with no scatter
