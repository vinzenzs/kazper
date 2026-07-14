# race-pacing-plan Specification

## Purpose
TBD - created by archiving change add-race-pacing-plan. Update Purpose after archive.
## Requirements
### Requirement: Per-leg pacing plan computed on read

The system SHALL compute a per-leg pacing plan over a stored race at read time
via `GET /races/{id}/pacing-plan` and SHALL NOT persist the computed result
(only manual overrides persist). Intensity thresholds MUST be read from the
`athlete_config` singleton (`ftp_watts`, `threshold_pace_sec_per_km`,
`threshold_swim_pace_sec_per_100m`); the endpoint takes no athlete query
parameters. The response SHALL carry, per leg, `ordinal`, `discipline`,
`expected_duration_min` (when set), a `source` marker
(`computed` | `override` | `none`), the discipline-appropriate target band,
`intensity_factor` and `estimated_tss` (when computable), and a `rationale`,
plus race-level `total_duration_min`, `estimated_tss_total`, `tss_complete`,
and a `missing_thresholds` union. `transition` legs SHALL carry no pacing
target and an `estimated_tss` of `0`; legs without `expected_duration_min`
SHALL carry no computed target or TSS and a rationale noting the unknown
duration.

#### Scenario: Plan reflects current thresholds and is not persisted

- **WHEN** a pacing plan is requested, `athlete_config.ftp_watts` is then
  changed, and the plan is requested again
- **THEN** each response reflects the `ftp_watts` value at its own read time
- **AND** no computed plan row is stored between calls

#### Scenario: Pacing plan for an unknown race

- **WHEN** a pacing plan is requested for a race id that does not exist
- **THEN** the response is `404` with `{"error":"race_not_found"}`

#### Scenario: Transition leg carries no pacing target

- **WHEN** a pacing plan is computed for a race containing a `transition` leg
- **THEN** that leg has no target band fields
- **AND** its `estimated_tss` is `0`
- **AND** its `rationale` notes that transitions are not paced

#### Scenario: Leg without a duration is not banded

- **WHEN** a pacing plan is computed for a bike leg whose
  `expected_duration_min` is null and which has no override
- **THEN** that leg has no target band, no `intensity_factor`, and no
  `estimated_tss`
- **AND** its `rationale` states the duration is unknown
- **AND** its `source` is `none`

### Requirement: Bike legs get a power band as a duration-banded percentage of FTP

For a `bike` leg with `expected_duration_min` set and `ftp_watts` configured,
the system SHALL compute `target_power_low_w` / `target_power_high_w` as
`round(ftp_watts × band)` (integer watts) using the leg-duration band: under
45 minutes → `90–100 %` FTP; `[45, 90)` → `83–90 %`; `[90, 180)` → `75–83 %`;
at least 180 minutes → `68–78 %`. The leg's `intensity_factor` SHALL be the
band midpoint and `estimated_tss = duration_hr × IF² × 100`.

#### Scenario: Full-distance bike leg bands at 68–78 % FTP

- **WHEN** `ftp_watts` is `265` and a bike leg has
  `expected_duration_min = 300`
- **THEN** the leg's `target_power_low_w` is `180` and `target_power_high_w`
  is `207`
- **AND** its `intensity_factor` is `0.73`
- **AND** its `estimated_tss` is `266.5` (5 h × 0.73² × 100, rounded to 1dp)

#### Scenario: Band boundary at 180 minutes

- **WHEN** two bike legs have `expected_duration_min` of `179` and `180`
- **THEN** the 179-minute leg uses the `75–83 %` band
- **AND** the 180-minute leg uses the `68–78 %` band

### Requirement: Run legs get a pace band as a duration-banded multiple of threshold pace

For a `run` leg with `expected_duration_min` set and
`threshold_pace_sec_per_km` configured, the system SHALL compute
`target_pace_low_sec_per_km` / `target_pace_high_sec_per_km` as
`threshold_pace × multiplier` (low = fast end) using the leg-duration band:
under 30 minutes → `×1.00–1.04`; `[30, 60)` → `×1.04–1.10`; `[60, 150)` →
`×1.10–1.18`; at least 150 minutes → `×1.18–1.28`. The leg's
`intensity_factor` SHALL be `1 / multiplier_midpoint` and
`estimated_tss = duration_hr × IF² × 100`. When the race contains a `bike` leg
with a lower ordinal, the run leg's `rationale` MUST note that the band
accounts for running off the bike (multisport context).

#### Scenario: 70.3-style run off the bike

- **WHEN** `threshold_pace_sec_per_km` is `270`, the race is
  swim → bike → run, and the run leg has `expected_duration_min = 100`
- **THEN** the run leg's `target_pace_low_sec_per_km` is `297.0` and
  `target_pace_high_sec_per_km` is `318.6`
- **AND** its `intensity_factor` is `0.88`
- **AND** its `rationale` mentions that the target comes after the bike

#### Scenario: Standalone short run is near threshold

- **WHEN** a race consists of a single run leg with
  `expected_duration_min = 25` and `threshold_pace_sec_per_km` is `270`
- **THEN** the leg's pace band is `270.0`–`280.8` sec/km (×1.00–×1.04)
- **AND** its `rationale` does not mention a preceding bike leg

### Requirement: Swim legs get a pace band per 100 m relative to CSS

For a `swim` leg with `expected_duration_min` set and
`threshold_swim_pace_sec_per_100m` configured, the system SHALL compute
`target_pace_low_sec_per_100m` / `target_pace_high_sec_per_100m` as
`css × multiplier` using the leg-duration band: under 20 minutes →
`×1.00–1.05`; `[20, 45)` → `×1.03–1.08`; at least 45 minutes → `×1.06–1.12`.
The leg's `intensity_factor` SHALL be `1 / multiplier_midpoint` and
`estimated_tss = duration_hr × IF³ × 100` (sTSS convention).

#### Scenario: Long-course swim bands at 1.06–1.12 × CSS

- **WHEN** `threshold_swim_pace_sec_per_100m` is `105` and a swim leg has
  `expected_duration_min = 70`
- **THEN** the leg's `target_pace_low_sec_per_100m` is `111.3` and
  `target_pace_high_sec_per_100m` is `117.6`
- **AND** its `intensity_factor` is `0.92`
- **AND** its `estimated_tss` is `90.1`

### Requirement: Missing thresholds degrade the affected legs, not the request

When the threshold a leg's computation needs is unset on `athlete_config`, the
system SHALL still return `200` with that leg's target band,
`intensity_factor`, and `estimated_tss` omitted, a `missing_thresholds` array
naming the unset field(s) on the leg, and a rationale stating what is missing.
The race-level `missing_thresholds` SHALL be the union over legs. The system
SHALL NOT fail the whole plan because one threshold is unset; legs whose
thresholds are configured MUST still be fully computed. `other`-discipline
legs SHALL carry no computed target (no threshold model), with `source: none`.

#### Scenario: FTP unset degrades only the bike leg

- **WHEN** `athlete_config` has `threshold_pace_sec_per_km` set but no
  `ftp_watts`, and the race has a bike leg and a run leg
- **THEN** the response is `200`
- **AND** the bike leg has no `target_power_low_w`/`target_power_high_w` and
  its `missing_thresholds` is `["ftp_watts"]`
- **AND** the run leg carries its full computed pace band
- **AND** the race-level `missing_thresholds` contains `ftp_watts`

#### Scenario: No config at all still returns a plan skeleton

- **WHEN** no `athlete_config` row exists and a pacing plan is requested
- **THEN** the response is `200` with every swim/bike/run leg uncomputed and
  its missing threshold named
- **AND** a leg with a stored override still reports the override's targets
  with `source: override`

### Requirement: Per-leg manual overrides persist and win over the computed band

The system SHALL persist per-leg pacing overrides keyed by
`(race_id, ordinal)` — surviving the wholesale leg replacement performed by
`PATCH /races/{id}` — and cascade-delete them with the race.
`PUT /races/{id}/pacing-plan/overrides/{ordinal}` SHALL full-replace the
override for that leg and `DELETE /races/{id}/pacing-plan/overrides/{ordinal}`
SHALL remove it (the leg reverts to computed on the next read). An override
MUST populate exactly one unit family (both `low` and `high`), matching the
leg's discipline: `target_power_low_w`/`target_power_high_w` for `bike`,
`target_pace_low_sec_per_km`/`target_pace_high_sec_per_km` for `run`,
`target_pace_low_sec_per_100m`/`target_pace_high_sec_per_100m` for `swim`; an
optional `note` MAY accompany it. Validation errors use documented codes:
`404 race_not_found`, `404 leg_not_found` (no leg with that ordinal),
`400 override_discipline_mismatch` (family does not match the leg's
discipline, including any override on `transition`/`other` legs),
`400 override_target_required` / `400 override_unit_conflict` (zero or more
than one family), `400 override_band_invalid` (non-positive values or
`low > high`). In the plan, an overridden leg SHALL report the override's
band with `source: "override"` and a rationale noting the manual override;
when the relevant threshold is set, `intensity_factor` and `estimated_tss`
SHALL be re-derived from the override band midpoint. An override whose unit
family no longer matches the leg's current discipline (after a leg edit)
SHALL be ignored at read time with a rationale note, never applied cross-unit.

#### Scenario: Override replaces the computed bike band

- **WHEN** the client PUTs
  `{"target_power_low_w":190,"target_power_high_w":200}` for a bike leg's
  ordinal and then requests the pacing plan
- **THEN** that leg reports `target_power_low_w: 190` and
  `target_power_high_w: 200` with `source: "override"`
- **AND** its `rationale` notes the manual override

#### Scenario: Deleting an override reverts to computed

- **WHEN** an override exists for a leg and the client DELETEs
  `/races/{id}/pacing-plan/overrides/{ordinal}`
- **THEN** the response is `204`
- **AND** the next pacing plan reports that leg with `source: "computed"` and
  the duration-banded values

#### Scenario: Override survives a wholesale leg replacement

- **WHEN** an override exists for ordinal `2` (a bike leg) and the client
  PATCHes the race with a `legs` array that still contains a bike leg at
  ordinal `2`
- **THEN** the next pacing plan still applies the override to leg `2`

#### Scenario: Discipline mismatch is rejected at write and ignored at read

- **WHEN** the client PUTs a power-family override for a `run` leg's ordinal
- **THEN** the response is `400` with `{"error":"override_discipline_mismatch"}`
- **WHEN** a stored bike-power override's leg is later replaced by a `run` leg
  at the same ordinal
- **THEN** the pacing plan ignores the override for that leg (`source:
  "computed"`) and the rationale notes the ignored mismatched override

#### Scenario: Unknown ordinal is a 404

- **WHEN** the client PUTs an override for an ordinal no leg of the race has
- **THEN** the response is `404` with `{"error":"leg_not_found"}`

#### Scenario: Idempotency-Key on the override PUT is rejected

- **WHEN** the client supplies an `Idempotency-Key` header on
  `PUT /races/{id}/pacing-plan/overrides/{ordinal}`
- **THEN** the response is `400` with
  `{"error":"idempotency_unsupported_for_put"}`
- **AND** no override row is created or modified

### Requirement: Race-level estimated TSS total is honest about coverage

The pacing-plan response SHALL carry `estimated_tss_total` as the sum of
per-leg `estimated_tss` values, and a boolean `tss_complete` that is `true`
only when every `swim`/`bike`/`run` leg produced an estimate (transition legs
never count against it; `other` legs and uncomputable legs make it `false`).

#### Scenario: Full triathlon sums leg TSS

- **WHEN** every swim/bike/run leg of a race has a duration and its threshold
  is configured
- **THEN** `estimated_tss_total` equals the sum of the legs' `estimated_tss`
- **AND** `tss_complete` is `true`

#### Scenario: An uncomputable leg flags the total as incomplete

- **WHEN** the bike leg cannot be estimated because `ftp_watts` is unset
- **THEN** `estimated_tss_total` sums only the computable legs
- **AND** `tss_complete` is `false`

### Requirement: Unit isolation across power and pace fields

The pacing-plan response SHALL keep power (`_w`), run pace (`_sec_per_km`),
and swim pace (`_sec_per_100m`) as distinct named fields and SHALL NOT merge
them into a shared target structure or attach a pace field to a bike leg (or
vice versa).

#### Scenario: Distinct unit fields per discipline

- **WHEN** a pacing plan is returned for a swim + bike + run race
- **THEN** power values appear only under `*_w` fields on the bike leg, run
  pace only under `*_sec_per_km` fields on the run leg, and swim pace only
  under `*_sec_per_100m` fields on the swim leg
- **AND** the bike leg's JSON contains no `sec_per_km` or `sec_per_100m` key

### Requirement: Numeric outputs are rounded at the response boundary

The system SHALL round pacing-plan numbers only when serializing (storage and
intermediate math at full precision): power targets to integer watts, pace
targets and `estimated_tss`/`estimated_tss_total` to one decimal place via
`numfmt.Round1`, and `intensity_factor` to two decimal places via
`numfmt.Round2` (matching the workouts capability's `intensity_factor`
precedent).

#### Scenario: IF is 2dp, TSS is 1dp

- **WHEN** a bike leg's band midpoint is `0.73` over 5 hours
- **THEN** the response's `intensity_factor` is `0.73` (two decimals)
- **AND** its `estimated_tss` is `266.5` (one decimal)

