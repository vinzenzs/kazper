## ADDED Requirements

### Requirement: Work intervals are detected from a stored power stream on read

The system SHALL expose `GET /api/v1/workouts/{id}/intervals` detecting sustained work efforts in
the workout's stored 1 Hz power stream with a deterministic, parameter-free procedure: a 30-second
centered rolling mean, a work/rest threshold derived from the ride's own smoothed power
distribution by Otsu's method (reported as `threshold_w`), merging of work spans separated by 30
seconds or less, and discarding of assembled spans shorter than 60 seconds. The response SHALL
carry each interval's ordinal, `start_s`/`end_s`/`duration_s`, `avg_w`, `max_w`, and `kj`, the
rest gaps between intervals, and a summary (`count`, `work_total_s`, `mean_effort_s`,
`mean_effort_w`), rounding kilojoules to 1 decimal at the boundary only. When the smoothed
distribution is not meaningfully bimodal the response SHALL be `200` with `threshold_w: null`,
`intervals: []`, and `reason: "no_distinct_efforts"`. The computation SHALL persist nothing.
An unknown workout SHALL return `404 workout_not_found`; no stored streams
`404 streams_not_found`; stored streams without power `404 power_stream_missing`.

#### Scenario: A structured ride returns its efforts

- **WHEN** the stored power stream contains five sustained supra-threshold efforts separated by
  clear recoveries
- **THEN** the response carries five intervals in ride order with durations, average/max power,
  and the derived `threshold_w`

#### Scenario: Brief lulls inside an effort do not split it

- **WHEN** an effort contains a sub-threshold dip of 30 seconds or less
- **THEN** it is returned as a single interval spanning the dip

#### Scenario: A steady ride reports no distinct efforts

- **WHEN** the ride's smoothed power distribution has no meaningful work/rest separation
- **THEN** the response is `200` with `intervals: []`, `threshold_w: null`, and
  `reason: "no_distinct_efforts"`

#### Scenario: Sub-minute bursts are not intervals

- **WHEN** a supra-threshold span lasts less than 60 seconds after gap-merging
- **THEN** it does not appear in `intervals`

### Requirement: Detected intervals are readable over MCP

The system SHALL expose a `detect_intervals` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/intervals` and forwarding the body verbatim — the detected-interval
list is compact structured data, unlike raw series, and is returned in full.

#### Scenario: Agent reads a ride's detected structure in one call

- **WHEN** the agent invokes `detect_intervals` with a workout id
- **THEN** the tool issues one GET and returns the intervals, rests, and summary verbatim
