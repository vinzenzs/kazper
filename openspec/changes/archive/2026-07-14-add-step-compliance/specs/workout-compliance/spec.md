# workout-compliance Specification (delta)

## ADDED Requirements

### Requirement: A read scores per-step compliance for a completed workout against its linked template

The system SHALL expose `GET /workouts/{id}/compliance` (authenticated),
computing on read — persisting nothing — a per-step execution score for a
completed workout against the template it is linked to via `template_id`. The
compared-against program SHALL be the workout's **effective program** (template
steps with slot `target_overrides`/`duration_overrides` applied and the
athlete-config zone→absolute target-resolution pass, per the `training-plan`
capability) — never the raw template steps. Prerequisite failures SHALL return
structured errors per the `http-error-shape` invariant: `400
workout_id_invalid` for a malformed id, `404 not_found` when the workout does
not exist, `409 workout_not_completed` when `status != 'completed'`, `409
multisport_unsupported` for a multisport workout, `409 no_template_link` when
`template_id` is null, and `409 splits_missing` when the completed, linked
workout carries no splits. The multisport check SHALL precede the
template-link check.

#### Scenario: A structured completed workout is scored

- **WHEN** a completed workout with a `template_id` and one split per executed
  template step exists and the client calls `GET /workouts/{id}/compliance`
- **THEN** the response is `200` with `status: "scored"`, a per-step array, and
  an overall `score`
- **AND** nothing is written to the database

#### Scenario: Slot overrides shape the compared-against target

- **WHEN** the workout's plan slot carries a `target_overrides` entry for
  intent `interval`
- **THEN** each interval step's compliance is judged against the overridden
  target, not the raw template target

#### Scenario: Missing workout returns 404

- **WHEN** the client calls the endpoint with an unknown workout id
- **THEN** the response is `404` with `{"error":"not_found"}`

#### Scenario: A planned workout is rejected

- **WHEN** the workout exists but its `status` is `planned`
- **THEN** the response is `409` with `{"error":"workout_not_completed"}`

#### Scenario: A workout without a template link is rejected

- **WHEN** the workout is completed but `template_id` is null (e.g. an imported
  free ride)
- **THEN** the response is `409` with `{"error":"no_template_link"}`

#### Scenario: A multisport workout is rejected as unsupported

- **WHEN** the workout's sport is `multisport`
- **THEN** the response is `409` with `{"error":"multisport_unsupported"}`
- **AND** it is NOT reported as `no_template_link`

#### Scenario: A splitless workout is rejected

- **WHEN** the workout is completed and template-linked but has zero splits
- **THEN** the response is `409` with `{"error":"splits_missing"}`

### Requirement: Executed laps match template steps positionally after repeat expansion

The system SHALL expand the effective program's step tree into a flat
executed-step sequence — each `repeat` group contributing `count` consecutive
copies of its inner steps in order — and SHALL match split `i` to expanded step
`i` if and only if the split count equals the expanded step count. Each
expanded step SHALL carry a flat 0-based `step_index` and, for steps originating
inside a repeat group, its `iteration` and the group's `count` so individual
intervals are nameable. When the counts differ, the system SHALL NOT guess an
alignment: the response SHALL be `200` with `status: "unavailable"`,
`reason: "lap_count_mismatch"`, the `planned_steps` and `executed_laps` counts,
and no per-step results.

#### Scenario: Repeat groups expand positionally

- **WHEN** the template is `[warmup, repeat ×5 of (interval, recovery), cooldown]`
  and the completed workout carries exactly 12 splits
- **THEN** the response scores 12 steps in order — warmup, then interval/recovery
  alternating five times, then cooldown
- **AND** the third interval is identified by its repeat provenance
  (`iteration: 3` of `5`)

#### Scenario: Lap-count mismatch yields an explicit unavailable result

- **WHEN** the expanded program has 12 steps but the workout carries 9 splits
- **THEN** the response is `200` with `status: "unavailable"` and
  `reason: "lap_count_mismatch"`
- **AND** it reports `planned_steps: 12` and `executed_laps: 9`
- **AND** no per-step scores are returned

### Requirement: Each matched step is scored against its resolved target and planned duration

For each matched step the system SHALL select the actual metric from the
matched split by the resolved target kind — `power_w` (including a resolved
`power_zone`) from the lap's average power, `hr_bpm` (including a resolved
`hr_zone`) from the lap's average HR, `pace` as `1000 / avg_speed_mps` in
sec/km, `swim_pace` as `100 / avg_speed_mps` in sec/100m — and SHALL classify
the actual against the resolved `[low, high]` band as `in_band`, `under`, or
`over`, reporting the metric, band, actual, a signed `delta` from the violated
bound (0 when in band), and a `deviation_pct`. A step whose target kind is
`cadence`, `rpe`, or `none`, whose zone target did not resolve to an absolute
range, or whose required actual is absent on the split SHALL be reported as
unscorable with a reason rather than failing the request. The system SHALL also
score planned-vs-actual duration for `time`-duration steps (against the lap's
duration) and `distance`-duration steps (against the lap's distance), reporting
the ratio; `lap_button` and `open` steps SHALL NOT be duration-scored.

#### Scenario: An under-target power interval is reported with its delta

- **WHEN** an interval step's resolved target is `power_w` 250–265 W and the
  matched lap's average power is 230 W
- **THEN** that step's target classification is `under` with `delta: -20`
- **AND** the step reports the band, the actual, and a `deviation_pct`

#### Scenario: A pace target scores from lap speed

- **WHEN** a run step targets `pace` 270–285 sec/km and the matched lap's
  average speed is 3.5 m/s
- **THEN** the actual pace is `285.7` sec/km and the step classifies as `over`
  (slower than the band)

#### Scenario: An unresolved zone target is unscorable, not an error

- **WHEN** the athlete config is absent so an `hr_zone` target reaches the read
  unresolved
- **THEN** that step's target is reported unscorable with reason
  `zone_unresolved`
- **AND** the response remains `200` and other steps are scored normally

#### Scenario: A cadence target has no per-lap actual

- **WHEN** a bike step's primary target kind is `cadence`
- **THEN** that step's target is reported unscorable
- **AND** its duration is still scored when the step has a `time` duration

#### Scenario: Duration compliance is reported for time steps

- **WHEN** a step planned 180 s has a matched lap of 178 s
- **THEN** the step's duration result reports planned, actual, and a ratio of
  `1.0` (rounded at the boundary), classified in-band

### Requirement: An overall workout compliance score aggregates scored steps

The system SHALL report an overall `score` on a 0–100 scale as the
planned-duration-weighted mean of per-step scores over scorable steps, where a
step's score is 100 when in band and falls off linearly with `deviation_pct`
outside the band, combined with its duration score when both dimensions are
scorable. The response SHALL report `steps_scored` and `steps_in_band` counts.
When no step is scorable the overall `score` SHALL be null while per-step rows
are still returned. All numeric fields SHALL be rounded at the response
boundary per the `numfmt` convention.

#### Scenario: The overall score weights steps by planned duration

- **WHEN** a workout has a 600 s tempo step scored 60 and a 60 s recovery step
  scored 100, both scorable
- **THEN** the overall `score` is closer to 60 than a plain average
  (approximately 63.6, rounded at the boundary)

#### Scenario: A workout with no scorable targets reports a null score

- **WHEN** every step's target is `rpe` or `none` and no step has a scorable
  duration
- **THEN** `status` is `"scored"`, per-step rows are present, and `score` is
  null
- **AND** `steps_scored` is 0

### Requirement: MCP tool mirrors the compliance read 1:1

The MCP server SHALL expose a `workout_compliance` tool taking a `workout_id`,
issuing exactly one `GET /workouts/{id}/compliance` request and forwarding the
response body verbatim — including the `unavailable` shape and structured error
bodies. The tool SHALL be registered in the shared agent-tools registry as a
read-tier tool, and its announced input schema SHALL be captured in the
announced-schema golden baseline.

#### Scenario: workout_compliance issues one GET

- **WHEN** the agent calls `workout_compliance` with a workout id
- **THEN** the MCP server issues exactly one `GET /workouts/{id}/compliance`
- **AND** the tool result is the REST response verbatim

#### Scenario: The unavailable shape reaches the agent

- **WHEN** the underlying read returns `status: "unavailable"` with
  `reason: "lap_count_mismatch"`
- **THEN** the tool result carries that body verbatim so the agent can explain
  why no score exists
