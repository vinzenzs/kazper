## MODIFIED Requirements

### Requirement: A template's steps are a validated structured program

A template's `steps` SHALL be an ordered, non-empty array of nodes. Each node is
either a single executable step or a repeat group. A single step SHALL carry an
`intent` (`warmup`, `active`, `interval`, `recovery`, `rest`, or `cooldown`),
exactly one `duration` (`{kind:"time",seconds}` with `seconds > 0`,
`{kind:"distance",meters}` with `meters > 0`, `{kind:"lap_button"}`, or
`{kind:"open"}`), and a `target` whose `kind` is one of `none`, `hr_zone`,
`power_zone`, `pace`, `hr_bpm`, `power_w`, or `rpe`; an optional free-text
`note` MAY be present. A step MAY additionally carry an optional
`secondary_target` (the same Target shape) **only on bike-sport templates**;
when present, its `kind` SHALL NOT be `none`, it SHALL be in a different metric
family than the primary `target` (power = `power_zone`/`power_w`, hr =
`hr_zone`/`hr_bpm`, pace, rpe), and it SHALL be validated by the same Target
validator. A repeat group SHALL carry a `count >= 2` and a non-empty `steps`
array of single steps only — repeat groups SHALL NOT nest. The system SHALL
validate this structure on every write at the service layer and reject malformed
steps with a sentinel error mapped to a 1:1 API error code.

#### Scenario: A valid structured template is accepted

- **WHEN** `POST /workout-templates` is called with a `run` template whose steps
  are `[warmup time 600s @ hr_zone 1–2, repeat ×5 of (interval time 180s @
  power_zone 4–4, recovery time 120s @ hr_zone 1), cooldown time 300s @ hr_zone 1]`
- **THEN** the template is persisted and returned with a generated `id` and the
  steps echoed verbatim

#### Scenario: A bike step accepts a primary + secondary target

- **WHEN** `POST /workout-templates` is called with a `bike` template whose
  interval step has primary `{kind:"power_zone", low:4, high:4}` and
  `secondary_target {kind:"hr_zone", low:3, high:3}`
- **THEN** the template is persisted and both targets are echoed verbatim

#### Scenario: secondary_target on a non-bike step is rejected

- **WHEN** a `run` or `swim` template supplies a step with a `secondary_target`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: A same-family or none secondary is rejected

- **WHEN** a bike step supplies a `secondary_target` whose `kind` is `none`, or
  whose metric family matches the primary target (e.g. primary `power_zone` with
  secondary `power_w`)
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: Empty steps are rejected

- **WHEN** a create or patch supplies `steps: []`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: Nested repeat groups are rejected

- **WHEN** a write supplies a `repeat` node whose `steps` contain another
  `repeat` node
- **THEN** the response is a validation error identifying the nesting violation

#### Scenario: Out-of-range target zones are rejected

- **WHEN** a step supplies a `hr_zone` or `power_zone` target with a bound
  outside `1..5`, or a `target` whose `low` exceeds its `high`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An unknown duration or target kind is rejected

- **WHEN** a step supplies a `duration` or `target` with an unrecognized `kind`
- **THEN** the response is a validation error and nothing is persisted
