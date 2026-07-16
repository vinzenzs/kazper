## MODIFIED Requirements

### Requirement: Config is the capture-only source of physiology; it consumes nothing in this change

The system SHALL treat `athlete-config` as the single source of truth for athlete
physiology. Its **zone-boundary fields** (`power_zone_*_max`, `hr_zone_*_max`)
SHALL be consumed by the `training-plan` capability's effective-program
resolution to expand zone-reference workout targets into absolute `power_w`/
`hr_bpm` ranges. Its **`ftp_watts` field** SHALL additionally be consumed by the
`workouts` capability to derive a bike workout's `intensity_factor` as
`normalized_power_w / ftp_watts` when that workout has `normalized_power_w` set
but no caller-supplied `intensity_factor` (see the `workouts` spec for the full
gate). Its **threshold fields** — `threshold_pace_sec_per_km`,
`threshold_swim_pace_sec_per_100m`, `lactate_threshold_hr`, and `threshold_hr`
(preferring `lactate_threshold_hr` when both HR fields are set) — SHALL
additionally be consumed by the `workouts` capability's per-sport TSS
derivation (rTSS, sTSS, hrTSS; see the `workouts` spec for the precedence and
gates), with `ftp_watts` participating transitively through the derived
`intensity_factor` in power-based TSS. All TSS-derivation consumption is
fail-open: an unset threshold never fails a workout write. Its threshold fields
`ftp_watts`, `threshold_pace_sec_per_km`, and `threshold_swim_pace_sec_per_100m`
SHALL additionally be consumed by the `race-pacing-plan` capability's
compute-on-read per-leg pacing targets (bike power band, run pace band, swim
pace band); an unset threshold degrades the affected legs of that plan rather
than erroring (see the `race-pacing-plan` spec). Its **`ftp_watts` field** (via
the effective-config resolution) SHALL additionally be consumed by the
`workout-fuel` capability's compute-on-read workout fueling plan to estimate a
planned session's work and carbohydrate burn; a missing FTP degrades that plan
to duration-based intake guidance rather than erroring (see the `workout-fuel`
spec). Beyond those consumptions, the config SHALL remain otherwise-unconsumed:
it does NOT relate the workouts capability's stored `secs_in_zone_*` to these
zone boundaries, and does NOT feed any value into the race-fueling/raceprep
intensity or carb-load math. Those remaining consumptions are explicit
follow-ups outside this change.

#### Scenario: Zone boundaries feed workout target resolution

- **WHEN** `athlete_config.power_zone_4_max` is set
- **AND** a planned workout has a step targeting `power_zone 4`
- **THEN** that step's effective-program target resolves to a `power_w` range
  bounded by the configured zone-4 boundary

#### Scenario: Storing FTP derives intensity_factor for a qualifying bike workout

- **WHEN** `athlete_config.ftp_watts` is set
- **AND** a `bike` workout is created with `normalized_power_w` set and no caller-supplied `intensity_factor`
- **THEN** that workout's `intensity_factor` is computed as `normalized_power_w / ftp_watts` (rounded to 2dp) and stored
- **AND** a workout that fails the gate (non-bike sport, missing `normalized_power_w`, or a caller-supplied `intensity_factor`) is unaffected

#### Scenario: Threshold fields feed per-sport TSS derivation

- **WHEN** `threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m`, and `lactate_threshold_hr` are set
- **AND** completed run/swim/HR-only workouts are created without a caller-supplied `tss`
- **THEN** the `workouts` capability derives rTSS, sTSS, and hrTSS respectively against those thresholds (per the `workouts` spec precedence)
- **AND** clearing a threshold makes the corresponding method fall through without failing any workout write

#### Scenario: Thresholds feed the race pacing plan

- **WHEN** `athlete_config.ftp_watts` is set
- **AND** the client requests `GET /races/{id}/pacing-plan` for a race with a
  bike leg carrying an expected duration
- **THEN** that leg's target power band derives from the configured
  `ftp_watts`
- **AND** updating `ftp_watts` changes the band on the next pacing-plan read
  (compute-on-read, nothing stored)

#### Scenario: FTP feeds the workout fueling plan

- **WHEN** effective `ftp_watts` resolves and a planned ride carries planned TSS
- **AND** the client requests `GET /workouts/{id}/fueling-plan`
- **THEN** the plan's estimated work derives from planned TSS × effective FTP
- **AND** an unset FTP degrades the plan to duration-based intake guidance
  rather than erroring

#### Scenario: Config is not merged into summary totals

- **WHEN** any config field is set
- **AND** the client calls `GET /summary/daily`
- **THEN** no `athlete_config` field appears in the summary `totals` (unit isolation preserved)
