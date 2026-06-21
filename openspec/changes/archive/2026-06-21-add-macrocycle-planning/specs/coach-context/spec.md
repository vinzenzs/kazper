## ADDED Requirements

### Requirement: Training context surfaces the current macrocycle

The system SHALL extend `GET /context/training` with an optional `macrocycle` block carrying
the season covering the anchor date, so the coach knows where today sits in the yearly
progression in the same grounding call. When a macrocycle's `[start_date, end_date]`
inclusive covers the anchor date, the block SHALL include the season's `id`, `name`,
`start_date`, `end_date`, its race anchor (`race_id`, `race_name`, `race_date`, and a derived
`days_to_race` = `race_date − anchor_date` in whole days, present only when the season is
race-anchored), and the current period's position in the progression (`current_phase_ordinal`
and `total_periods`). `current_phase_ordinal` SHALL be the `macrocycle_ordinal` of the
covering phase when that phase belongs to this macrocycle (else null), and `total_periods`
the count of phases linked to the macrocycle. When two macrocycles cover the anchor date, the
resolver SHALL pick the most-recently-updated one (mirroring the covering-phase rule). When no
macrocycle covers the anchor date, `macrocycle` SHALL serialize as null, not an error. The
block is composition-only — it does not affect adherence, the covering phase, or any other
field in the bundle.

#### Scenario: Training read includes the covering macrocycle

- **WHEN** a macrocycle covers `2026-03-15`, is anchored to a race dated `2026-09-27`, and the covering phase is its ordinal-2 of 6 member phases
- **AND** the client GETs `/context/training?date=2026-03-15`
- **THEN** the response `macrocycle` block carries the season identity, `race_name`, `race_date`, `days_to_race` equal to the whole-day gap to `2026-09-27`, `current_phase_ordinal = 2`, and `total_periods = 6`

#### Scenario: Unanchored season omits the race fields

- **WHEN** the covering macrocycle has `race_id = NULL`
- **THEN** the `macrocycle` block is present with the season identity and period position, and `race_id`/`race_name`/`race_date`/`days_to_race` are null

#### Scenario: No covering macrocycle serializes as null

- **WHEN** no macrocycle covers the anchor date
- **THEN** the `macrocycle` field is null and the rest of the training bundle is unaffected

#### Scenario: Overlapping seasons resolve to the most-recently-updated

- **WHEN** two macrocycles both cover the anchor date — season A updated at T1 and season B updated at T2 > T1
- **THEN** the `macrocycle` block reflects season B

#### Scenario: Covering phase outside the season leaves the ordinal null

- **WHEN** a macrocycle covers the anchor date but the covering phase is not linked to it (its `macrocycle_id` differs or is null)
- **THEN** the `macrocycle` block is present with `current_phase_ordinal = null` while `total_periods` still reflects the season's member count
