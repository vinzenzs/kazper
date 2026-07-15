# coach-context Specification

## Purpose
TBD - created by archiving change add-coach-context-endpoints. Update Purpose after archive.
## Requirements
### Requirement: Training context aggregate read

The system SHALL expose `GET /context/training` returning a single composition-only bundle for grounding training advice: the training phase covering the anchor date — **including that phase's `methodology` Markdown prose when set, so the coach has the cited "why" of the current phase in the same call** — the latest fitness snapshot on/before that date within the lookback window (VO2max, acute/chronic load, training status, race predictors), the derived ACWR (acute ÷ chronic load) when both loads are present, the athlete's physiology config block (FTP, thresholds, HR/power zones) when set, a derived `watts_per_kg` (FTP ÷ the latest bodyweight on/before the anchor date) when both inputs are present, a recent-load summary and the recent completed workouts over a lookback window (default 14 days), and the upcoming planned workouts over a lookahead window (default 7 days). It SHALL accept `date` (YYYY-MM-DD, defaulting to today in the configured zone), `tz`, `lookback_days`, and `lookahead_days`; lookback and lookahead SHALL be clamped to sane bounds. The bundle SHALL be built from existing read repos in parallel with no partial result on error, and numeric fields SHALL be rounded at the response boundary. Absent snapshots (no fitness/phase/athlete-config/bodyweight) SHALL serialize as null, not errors; a covering phase with no `methodology` SHALL serialize that field as null; `watts_per_kg` SHALL be null whenever FTP or bodyweight is missing.

#### Scenario: Grounded training read

- **WHEN** the client GETs `/context/training?date=2026-06-14`
- **THEN** the response includes the covering phase (or null), the latest fitness snapshot (or null), `acwr` when acute and chronic load are both present, the `athlete_config` block when set, `watts_per_kg` when FTP and bodyweight are both present, a `recent_load` summary plus recent completed workouts within the lookback, and upcoming planned workouts within the lookahead

#### Scenario: The covering phase carries its methodology

- **WHEN** the phase covering the anchor date has a `methodology` set and the client GETs `/context/training`
- **THEN** the phase slice of the bundle includes that `methodology` prose verbatim

#### Scenario: A phase without methodology serializes null

- **WHEN** the covering phase has no `methodology`
- **THEN** the phase slice's `methodology` field is null and the response is otherwise unchanged

#### Scenario: Athlete config and W/kg are surfaced when present

- **WHEN** an athlete_config row with an FTP is set and a bodyweight has been logged on/before the anchor date, and the client GETs `/context/training`
- **THEN** the response includes the `athlete_config` block and a `watts_per_kg` equal to FTP ÷ the latest bodyweight in kg, rounded at the response boundary

#### Scenario: W/kg is null when an input is missing

- **WHEN** athlete_config has no FTP, or no bodyweight exists on/before the anchor date
- **THEN** `watts_per_kg` is null and the bundle is otherwise returned normally (200, not an error)

#### Scenario: Quiet history is not an error

- **WHEN** there are no workouts, fitness, phase, athlete-config, or bodyweight for the window
- **THEN** the response is 200 with null/empty fields, not an error

#### Scenario: Unbounded windows are clamped

- **WHEN** the client passes an absurd `lookback_days`
- **THEN** the server clamps it to the maximum rather than scanning unboundedly

### Requirement: Recovery context aggregate read

The system SHALL expose `GET /context/recovery` returning the latest recovery snapshot on/before the anchor date within the window (sleep, HRV, resting HR, body battery, training readiness, …) plus the recent trend of snapshots over a window (default 7 days). It SHALL accept `date` (YYYY-MM-DD, defaulting to today) and `days` (clamped). Absent data SHALL serialize as null/empty, not an error.

#### Scenario: Recovery trend read

- **WHEN** the client GETs `/context/recovery?date=2026-06-14&days=7`
- **THEN** the response includes `latest` (the most recent snapshot on/before the date, or null) and `recent` (the snapshots over the window, ascending)

### Requirement: Aggregate context reads are exposed as MCP tools

The MCP server SHALL expose `get_training_context` and `get_recovery_context` tools, each performing exactly one loopback call to the corresponding REST endpoint under the caller's bearer, and both names SHALL appear in the server's announced tool surface. The names SHALL be identical to those the in-app chat coach uses, so the two AI surfaces do not diverge.

#### Scenario: MCP announces the aggregate context tools

- **WHEN** an MCP client lists tools
- **THEN** `get_training_context` and `get_recovery_context` are announced alongside the existing surface

### Requirement: The training-context load summary decomposes multisport workouts by segment sport

The `GET /context/training` recent-load summary SHALL count a `multisport`
workout once per **non-transition segment sport** of its referenced multisport
template in the `by_sport` breakdown — so a swim→bike→run brick contributes one
each to `swim`, `bike`, and `run` rather than a single `multisport` entry. The
segment sports SHALL be resolved from the workout's `multisport_template_id`. The decomposition SHALL apply only to the `by_sport`
map; the summary's `count`, `total_duration_min`, and `total_kcal` SHALL still
treat the brick as a single session (one count, one window, one burn). When the
multisport template cannot be resolved (the repo is unavailable, the template was
deleted, or the load fails), the workout SHALL fall back to a single `multisport`
entry in `by_sport`, and the aggregate read SHALL NOT error. Non-multisport
workouts SHALL be counted under their own sport exactly as before.

#### Scenario: A brick credits each of its segment sports

- **WHEN** the recent-load window contains a `multisport` workout whose template
  has swim, transition, bike, transition, and run segments
- **THEN** `by_sport` shows `swim`, `bike`, and `run` each incremented by one (and
  no `transition` or `multisport` entry from that workout)
- **AND** `count` increases by exactly one for that workout

#### Scenario: An unresolvable multisport workout falls back to the multisport bucket

- **WHEN** a `multisport` workout is in the window but its template cannot be
  resolved
- **THEN** `by_sport` shows a single `multisport` entry for it and the response is
  returned without error

#### Scenario: Single-sport workouts are unaffected

- **WHEN** the window contains only single-sport workouts
- **THEN** `by_sport` counts each under its own sport exactly as before

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

### Requirement: The training context response carries active coach memory

The `GET /context/training` aggregator response SHALL include a `memory` block listing
active coach-memory items: every standing item (`status = active`, not expired, kind in
`fact | preference | constraint | observation`) plus any `recommendation` whose `date`
falls in the aggregator's lookback window. Each item past its `review_at`
(`review_at <= today`) SHALL carry `needs_review: true`. Archived and expired items SHALL be
excluded. The aggregator performs no synthesis over memory — items are returned verbatim.
This is what lets the external MCP agent ground on what the in-app coach was told (and
vice-versa) without sharing conversation transcripts.

#### Scenario: Memory grounds the training context

- **WHEN** an active `preference` ("prefers gels over drink mix") exists and the client requests `/context/training`
- **THEN** the `memory` block includes that preference

#### Scenario: Recommendations are window-scoped in training context

- **WHEN** a `recommendation` is dated outside the lookback window
- **THEN** it is absent from the `memory` block, while dateless standing items remain present

### Requirement: The training context carries detection, source policy, and effective values beside the config

The `/api/v1/context/training` bundle SHALL include, beside the existing athlete-config block:
`garmin_detected` (the latest detected values with `detected_at`; null when none),
`threshold_sources` (the active garmin-sourced field list; empty when all-manual), and an
`effective` block (the resolved values with per-field `source` annotations) — so the coach reads
configured, detected, and live-in-computations in one call, names drift, and flips a source or
proposes a deliberate update from there. Absent pieces SHALL serialize as null/empty, never as
errors, and the rest of the bundle SHALL be unaffected.

#### Scenario: Drift and policy are visible in one read

- **WHEN** the config holds `ftp_watts: 278`, the detection holds 285, and `ftp_watts` is
  garmin-sourced
- **THEN** the bundle shows the configured 278, the detected 285 with `detected_at`,
  `threshold_sources: ["ftp_watts"]`, and an effective `ftp_watts: 285` annotated
  `source: "garmin"`

#### Scenario: No detection degrades to null

- **WHEN** no detection has been recorded and the policy is empty
- **THEN** `garmin_detected` is null, `threshold_sources` is empty, the effective block equals
  the config, and the bundle is otherwise unchanged

