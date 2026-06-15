## MODIFIED Requirements

### Requirement: A planned workout's effective program applies slot overrides to template steps

The system SHALL define a planned workout's **effective program** as its
template's steps with each step's `target` replaced when that step's `intent`
matches an entry in the workout's slot `target_overrides`, **and** each step's
`duration` replaced when that step's `intent` matches an entry in the workout's
slot `duration_overrides`. The two override lists SHALL be independent and SHALL
compose (a step whose intent matches both gets both its target and its duration
replaced). Steps whose intent matches no override SHALL be unchanged, and
overrides SHALL affect targets and durations of existing steps only, never step
structure (step count, order, or repeat-group counts).

After overrides are applied, the system SHALL perform a **target-resolution
pass** over the effective steps (including steps nested in repeat groups) using
the `athlete-config` singleton: a `power_zone` target SHALL be rewritten to a
`power_w` range and an `hr_zone` target to an `hr_bpm` range, where the lower
bound is the `Max` boundary of the zone below the lowest referenced zone (`0`
for zone 1) and the upper bound is the `Max` boundary of the highest referenced
zone. Targets of kind `pace`, `power_w`, `hr_bpm`, `rpe`, and `none` SHALL pass
through unchanged. `power_zone` resolution SHALL apply only to **bike** workouts,
because the athlete config's power zones are FTP/bike-derived; for any other
sport a `power_zone` target SHALL pass through unchanged. `hr_zone` resolution
SHALL apply across all sports. When the athlete config is absent, or a required
zone-boundary field for a referenced zone is unset, that zone target SHALL pass
through unchanged (deferring resolution to the watch) and resolution SHALL NOT
error.
Resolution SHALL be computed on read and SHALL NOT be persisted.

This effective program SHALL be the single representation that downstream
consumers use — both display and the Garmin compile path (`add-garmin-scheduling`)
SHALL build from effective steps, not raw template steps. The effective program
SHALL be resolved on read from the template, slot, and athlete config, not
snapshotted onto the workout row.

#### Scenario: Override replaces only the matching intent's duration

- **WHEN** a template is `[warmup 10min @hr_zone 1, active 55min @tempo, cooldown 10min @hr_zone 1]`
  and the slot overrides `active` with `duration {kind:"time", seconds:3600}`
- **THEN** the effective program has the `active` step lasting 60min
- **AND** the warmup and cooldown durations are unchanged at 10min
- **AND** the step structure and all targets are unchanged

#### Scenario: Duration and target overrides compose on the same intent

- **WHEN** a slot carries both a `target_overrides` entry and a
  `duration_overrides` entry for intent `interval`
- **THEN** the effective program's interval steps show both the overridden target
  and the overridden duration

#### Scenario: No overrides yields the template program verbatim

- **WHEN** a planned workout's slot has neither `target_overrides` nor
  `duration_overrides`
- **THEN** its effective program equals the template's steps with the
  target-resolution pass applied to any zone targets

#### Scenario: Power-zone target resolves to a watts range

- **WHEN** athlete config has `power_zone_3_max = 230` and `power_zone_4_max = 268`
  and an effective step's target is `{kind:"power_zone", low:4, high:4}`
- **THEN** the step's target becomes `{kind:"power_w", low:230, high:268}`

#### Scenario: HR-zone target resolves to a bpm range

- **WHEN** athlete config has `hr_zone_3_max = 167` and `hr_zone_4_max = 178`
  and an effective step's target is `{kind:"hr_zone", low:4, high:4}`
- **THEN** the step's target becomes `{kind:"hr_bpm", low:167, high:178}`

#### Scenario: Multi-zone reference spans the band edges

- **WHEN** athlete config has `power_zone_1_max = 140` and `power_zone_4_max = 268`
  and an effective step's target is `{kind:"power_zone", low:2, high:4}`
- **THEN** the step's target becomes `{kind:"power_w", low:140, high:268}`

#### Scenario: Zone targets nested in a repeat group resolve

- **WHEN** a repeat group contains a step targeting `{kind:"power_zone", low:5, high:5}`
- **THEN** that nested step's target is resolved to a `power_w` range
- **AND** the repeat group's count and structure are unchanged

#### Scenario: Run power-zone target passes through unresolved

- **WHEN** a **run** workout's effective step targets `{kind:"power_zone", low:4, high:4}`
  and athlete config defines power zones
- **THEN** the step's target remains `{kind:"power_zone", low:4, high:4}`
  (power zones are bike/FTP-derived and not applied to run)

#### Scenario: HR-zone targets resolve regardless of sport

- **WHEN** a **run** workout's effective step targets `{kind:"hr_zone", low:2, high:2}`
  and athlete config defines HR zones
- **THEN** the step's target resolves to an `hr_bpm` range

#### Scenario: Missing athlete config leaves zone targets unchanged

- **WHEN** no athlete config exists (or the referenced zone's `Max` field is unset)
  and an effective step's target is `{kind:"power_zone", low:4, high:4}`
- **THEN** the step's target remains `{kind:"power_zone", low:4, high:4}`
- **AND** no error is returned

### Requirement: A read endpoint exposes a planned workout's effective program

The system SHALL expose `GET /workouts/{id}/program` returning the effective steps
of a planned workout (resolved from its `template_id`, its slot's
`target_overrides`/`duration_overrides`, and the athlete config) together with
enough metadata to render it (sport, name). Zone-reference targets in the response
SHALL appear as their resolved absolute (`power_w`/`hr_bpm`) form when athlete
config permits resolution, and each resolved target SHALL carry an `origin`
label naming the source zone(s) (e.g. `"Z4"`). When the workout has no
`template_id`, the endpoint SHALL return its bare metadata with no steps rather
than an error. The endpoint SHALL require authentication.

#### Scenario: Program reflects the slot override

- **WHEN** a client `GET`s `/workouts/{id}/program` for a planned workout whose
  slot overrides the interval pace to `7:15`
- **THEN** the returned steps show the interval target as `pace 7:15`

#### Scenario: Program shows resolved zone targets with origin

- **WHEN** a client `GET`s `/workouts/{id}/program` for a planned workout whose
  interval step targets `power_zone 4` and athlete config defines that zone
- **THEN** the returned interval target is `power_w` with the resolved watts range
- **AND** the target carries an `origin` label of `"Z4"`

#### Scenario: A template-less planned workout returns metadata without steps

- **WHEN** a planned workout has no `template_id`
- **THEN** `GET /workouts/{id}/program` returns its sport and name with an empty
  step list and no error
