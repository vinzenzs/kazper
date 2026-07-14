## ADDED Requirements

### Requirement: Power-profile ranking against the Coggan tables

The system SHALL expose `GET /api/v1/workouts/power-profile?from=&to=&tz=&weight_kg=&sex=`
that ranks the athlete's windowed best power efforts against the embedded Coggan power-profile
tables. For each of the four benchmark anchors — 5 s (neuromuscular), 60 s (anaerobic), 300 s
(VO₂max), and 1200 s ranked against the functional-threshold column as a documented 20-minute
proxy — the response SHALL carry `duration_s`, `watts`, `w_per_kg`, the Coggan `category` band,
an interpolated `percentile` (0–100), and the contributing `workout_id` and `date`. The metric is
power only; there is no `sport` parameter. The endpoint is compute-on-read and SHALL persist
nothing. It SHALL reuse the shared window contract (`range_required` / `date_invalid` /
`range_invalid` / `range_too_large` with `max_days` / `tz_invalid`, capped at 400 days).

The `percentile` SHALL be obtained by linear interpolation between adjacent Coggan table rows and
is an ESTIMATE — the category band is the authoritative output; the percentile is a smooth
position within the table's W/kg span, clamped to `[0,100]`, not a claim about a real population
distribution.

The endpoint SHALL be advisory: it SHALL NOT read or write athlete-config, and the Coggan tables
are a fixed reference, not a personal calibration.

#### Scenario: A full profile ranks all four anchors

- **WHEN** the window holds best efforts at all four anchor durations and `weight_kg` is supplied
- **THEN** the response carries four `anchors` entries, each with `w_per_kg = watts / weight_kg`,
  a `category` band and an interpolated `percentile`, and `missing_anchors` is empty

#### Scenario: A missing anchor is named, not fabricated

- **WHEN** the window has no 5-second best effort
- **THEN** the `anchors` array omits the neuromuscular entry and `missing_anchors` lists it, and
  the remaining anchors still rank normally

#### Scenario: The 20-minute best is the FT proxy

- **WHEN** the 1200-second best is ranked
- **THEN** it is compared against the functional-threshold column with no 0.95 correction and the
  response/description identifies it as a 20-minute proxy

### Requirement: Power-profile weight resolution and sex selection

The system SHALL resolve the W/kg denominator in this order: the `weight_kg` query parameter
(which MUST be `> 0`, else `400 weight_kg_invalid`) as the highest-trust source; otherwise the
most-recent stored body-weight entry; otherwise `400 weight_data_missing`. The response SHALL
echo the resolved `weight_kg` and a `weight_source` of `param` or `stored` so the denominator is
auditable. The `sex` parameter SHALL select the Coggan table (`male` or `female`, defaulting to
`male` when omitted); any other value SHALL return `400 sex_invalid`.

#### Scenario: An explicit weight overrides the stored value

- **WHEN** `weight_kg=70` is supplied and a stored body-weight entry also exists
- **THEN** the ranking uses 70 kg and the response reports `weight_source: "param"`

#### Scenario: The stored weight is the fallback

- **WHEN** no `weight_kg` is supplied but a body-weight entry exists
- **THEN** the ranking uses the most-recent stored weight and reports `weight_source: "stored"`

#### Scenario: No weight anywhere is a 400

- **WHEN** no `weight_kg` is supplied and no body-weight entry has ever been logged
- **THEN** the response is `400` with `weight_data_missing`

#### Scenario: An unknown sex value is rejected

- **WHEN** `sex=other` is supplied
- **THEN** the response is `400` with `sex_invalid`

### Requirement: Power-profile rider phenotype

The response SHALL include a `phenotype` classifying the rider from the relative standing of the
four anchors — `sprinter`, `time_trialist`, `climber`, or `all_rounder`. The phenotype SHALL be
`null` unless all four anchors are present (a full profile is required to name a type). It is
advisory and SHALL NOT be persisted.

#### Scenario: A sprint-dominant profile is classified

- **WHEN** the 5-second and 60-second anchors rank far higher than the 300-second and
  1200-second anchors and all four are present
- **THEN** `phenotype` is `sprinter`

#### Scenario: An incomplete profile has no phenotype

- **WHEN** any of the four anchors is missing from the window
- **THEN** `phenotype` is `null`

### Requirement: Power-profile ranking is available over MCP

The system SHALL expose a `power_profile` MCP read tool wrapping
`GET /api/v1/workouts/power-profile` (one HTTP call, response body forwarded verbatim). The tool
description SHALL state the four anchor durations, the 20-minute FT proxy, and that the ranking is
advisory (category primary, percentile an estimate).

#### Scenario: The agent reads the profile in one call

- **WHEN** the agent invokes `power_profile` with a window and optional `weight_kg`/`sex`
- **THEN** the tool issues one GET and returns the ranking body verbatim
