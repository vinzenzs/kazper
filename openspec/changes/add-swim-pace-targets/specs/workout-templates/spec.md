## MODIFIED Requirements

### Requirement: A template's steps are a validated structured program

A template's `steps` SHALL be an ordered, non-empty array of nodes. Each node is
either a single executable step or a repeat group. A single step SHALL carry an
`intent` (`warmup`, `active`, `interval`, `recovery`, `rest`, or `cooldown`),
exactly one `duration` (`{kind:"time",seconds}` with `seconds > 0`,
`{kind:"distance",meters}` with `meters > 0`, `{kind:"lap_button"}`, or
`{kind:"open"}`), and a `target` whose `kind` is one of `none`, `hr_zone`,
`power_zone`, `pace`, `swim_pace`, `hr_bpm`, `power_w`, or `rpe`; an optional
free-text `note` MAY be present. A `swim_pace` target SHALL carry
`low_sec_per_100m`/`high_sec_per_100m` (positive, `low <= high`) and SHALL be
accepted only on swim-sport templates; conversely `pace` (`/km`) SHALL be
rejected on swim steps. A repeat group SHALL carry a `count >= 2` and a non-empty
`steps` array of single steps only — repeat groups SHALL NOT nest. The system
SHALL validate this structure on every write at the service layer and reject
malformed steps with a sentinel error mapped to a 1:1 API error code.

#### Scenario: A valid structured template is accepted

- **WHEN** `POST /workout-templates` is called with a `run` template whose steps
  are `[warmup time 600s @ hr_zone 1–2, repeat ×5 of (interval time 180s @
  power_zone 4–4, recovery time 120s @ hr_zone 1), cooldown time 300s @ hr_zone 1]`
- **THEN** the template is persisted and returned with a generated `id` and the
  steps echoed verbatim

#### Scenario: A swim template accepts a swim_pace target

- **WHEN** `POST /workout-templates` is called with a `swim` template whose
  interval step targets `{kind:"swim_pace", low_sec_per_100m:95, high_sec_per_100m:100}`
- **THEN** the template is persisted and the swim_pace target is echoed verbatim

#### Scenario: swim_pace on a non-swim template is rejected

- **WHEN** a `bike` or `run` template supplies a step with a `swim_pace` target
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: km pace on a swim template is rejected

- **WHEN** a `swim` template supplies a step with a `pace` (`low_sec_per_km`) target
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An invalid swim_pace range is rejected

- **WHEN** a swim step supplies a `swim_pace` target whose `low_sec_per_100m`
  exceeds its `high_sec_per_100m`, or a non-positive bound
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
