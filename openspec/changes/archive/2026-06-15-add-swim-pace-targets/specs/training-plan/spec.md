## MODIFIED Requirements

### Requirement: Plan slots carry optional per-intent target overrides

The system SHALL allow a `plan_slot` to carry an optional `target_overrides` list,
each entry a `{intent, target}` pair, stored as a JSONB column. The `intent` SHALL
be one of the workout-template step intents (`warmup`, `active`, `interval`,
`recovery`, `rest`, `cooldown`) and the `target` SHALL use the workout-templates
Target shape and be validated by the same validator (pace bounds positive, zones
within `1..5`, `low <= high`, and `swim_pace` bounds positive with
`low_sec_per_100m <= high_sec_per_100m`). The list SHALL contain at most one
entry per intent; a null or empty list means no overrides. Slot create and patch
SHALL accept `target_overrides`, the nested plan `GET` SHALL return it, and a
`PATCH` that supplies the list SHALL replace it wholesale (supplying `[]` clears
it, omitting it leaves it unchanged).

#### Scenario: A slot stores a pace override for its work intervals

- **WHEN** a slot referencing an interval template is created with
  `target_overrides: [{intent:"interval", target:{kind:"pace", low_sec_per_km:435, high_sec_per_km:435}}]`
- **THEN** the slot persists the override and the nested plan `GET` returns it

#### Scenario: A swim slot stores a swim_pace override

- **WHEN** a slot referencing a swim template is created with
  `target_overrides: [{intent:"interval", target:{kind:"swim_pace", low_sec_per_100m:92, high_sec_per_100m:96}}]`
- **THEN** the slot persists the override and the nested plan `GET` returns it
- **AND** the effective program carries the swim_pace target on matching steps

#### Scenario: Duplicate intent in the override list is rejected

- **WHEN** a slot write supplies two override entries with the same `intent`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An invalid override target is rejected

- **WHEN** an override supplies a `pace` target with `low_sec_per_km` greater than
  `high_sec_per_km`, or a zone outside `1..5`, or a `swim_pace` with inverted
  bounds, or an unknown target kind
- **THEN** the response is a validation error (the workout-templates Target
  validator) and nothing is persisted

#### Scenario: Patch replaces the override list wholesale

- **WHEN** a client `PATCH`es a slot with a new `target_overrides` list
- **THEN** the prior list is replaced entirely
- **AND** supplying `[]` clears all overrides while omitting the field leaves them unchanged
