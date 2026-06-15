## MODIFIED Requirements

### Requirement: Config is the capture-only source of physiology; it consumes nothing in this change

The system SHALL treat `athlete-config` as the single source of truth for athlete
physiology. Its **zone-boundary fields** (`power_zone_*_max`, `hr_zone_*_max`)
SHALL be consumed by the `training-plan` capability's effective-program
resolution to expand zone-reference workout targets into absolute `power_w`/
`hr_bpm` ranges. Beyond that resolution, the config SHALL remain
otherwise-unconsumed in this change: it does NOT derive `intensity_factor` from
`ftp_watts`, does NOT relate the workouts capability's stored `secs_in_zone_*` to
these zone boundaries, and does NOT feed any value into the race-fueling/raceprep
intensity or carb-load math. Those remaining consumptions are explicit follow-ups
outside this change.

#### Scenario: Zone boundaries feed workout target resolution

- **WHEN** `athlete_config.power_zone_4_max` is set
- **AND** a planned workout has a step targeting `power_zone 4`
- **THEN** that step's effective-program target resolves to a `power_w` range
  bounded by the configured zone-4 boundary

#### Scenario: Storing FTP does not back-fill workout intensity_factor

- **WHEN** `athlete_config.ftp_watts` is set and a workout with `normalized_power_w` set but `intensity_factor` NULL exists
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the workout's `intensity_factor` remains NULL (unchanged by this change)
- **AND** no computation of `normalized_power_w / ftp_watts` occurs

#### Scenario: Config is not merged into summary totals

- **WHEN** any config field is set
- **AND** the client calls `GET /summary/daily`
- **THEN** no `athlete_config` field appears in the summary `totals` (unit isolation preserved)
