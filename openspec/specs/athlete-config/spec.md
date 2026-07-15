# athlete-config Specification

## Purpose
TBD - created by archiving change add-garmin-athlete-config. Update Purpose after archive.
## Requirements
### Requirement: Single-row athlete physiology configuration

The system SHALL maintain exactly one `athlete_config` row representing the active user's physiology configuration — FTP, threshold heart rate and paces, max HR, lactate-threshold HR, and HR-zone (and optional power-zone) boundaries. The row is a singleton (fixed sentinel primary key, upsert-in-place), created lazily on first write, mirroring the `nutrition_goals` singleton shape. Every field is nullable: a NULL means "not configured / not provided by Garmin", distinct from a real zero.

#### Scenario: Config is absent until first write

- **WHEN** the client calls `GET /athlete-config` before any config has been set
- **THEN** the system returns `200 OK` with `{"athlete_config": null}`

#### Scenario: First PUT creates the config row

- **WHEN** the client calls `PUT /athlete-config` with a body containing any config fields
- **THEN** the system creates the single `athlete_config` row
- **AND** returns `200 OK` with the stored config object

#### Scenario: Subsequent PUT overwrites the config row

- **WHEN** the client calls `PUT /athlete-config` and a row already exists
- **THEN** the system replaces all config fields with the values from the request body
- **AND** absent fields are stored as null (cleared), matching `PUT /goals` full-replace semantics
- **AND** returns `200 OK` with the stored config object

### Requirement: Config carries threshold, max-HR, and zone-boundary fields

The system SHALL accept the following optional fields on `PUT /athlete-config`, persist them on the singleton row, and return the populated ones (nulls omitted) on `GET /athlete-config`. All fields are nullable and independent — any subset MAY be supplied.

Supported fields:

- `ftp_watts` (integer, cycling functional threshold power; must be `> 0` when present)
- `threshold_hr` (integer, functional threshold heart rate in bpm; `> 0`)
- `lactate_threshold_hr` (integer, Garmin lactate-threshold HR in bpm; `> 0`) — kept distinct from `threshold_hr` because Garmin exposes both
- `max_hr` (integer, bpm; `> 0`)
- `threshold_pace_sec_per_km` (number, run threshold pace in seconds per kilometre; `> 0`)
- `threshold_swim_pace_sec_per_100m` (number, swim threshold pace in seconds per 100 m; `> 0`)
- `hr_zone_1_max` … `hr_zone_5_max` (five integers, the upper HR bound of each zone in bpm; each `> 0` when present)
- `power_zone_1_max` … `power_zone_5_max` (five integers, the upper power bound of each zone in watts; optional; each `> 0` when present)

Zone boundaries are stored as each zone's *maximum* (upper bound); zone 1's lower bound is resting/0 and each subsequent zone's lower bound is the previous zone's max, so five maxima fully describe the boundaries.

#### Scenario: Partial config is accepted

- **WHEN** the client calls `PUT /athlete-config` with only `{"ftp_watts": 265, "max_hr": 188}`
- **THEN** the system stores those two fields
- **AND** stores all other config columns as null
- **AND** the response includes only the populated fields (nulls omitted)

#### Scenario: HR-zone boundaries are stored and returned

- **WHEN** the client puts `{"hr_zone_1_max": 120, "hr_zone_2_max": 140, "hr_zone_3_max": 155, "hr_zone_4_max": 168, "hr_zone_5_max": 182}`
- **THEN** the system stores all five HR-zone maxima
- **AND** `GET /athlete-config` returns them on the config object

#### Scenario: Power zones are optional and omitted when absent

- **WHEN** the stored config has HR-zone boundaries set but no power-zone boundaries
- **AND** the client calls `GET /athlete-config`
- **THEN** the response includes the `hr_zone_*` fields
- **AND** the response omits every `power_zone_*` key (nulls omitted)

#### Scenario: Negative or non-numeric values are rejected

- **WHEN** the client supplies any field whose value is negative, NaN, or non-numeric (for example `{"ftp_watts": -10}`)
- **THEN** the system returns `400 Bad Request` with `{"error":"athlete_config_value_invalid","field":"<which>"}`
- **AND** the request is not partially applied

### Requirement: PUT /athlete-config rejects an idempotency key

The system SHALL reject `PUT /athlete-config` when the request carries an `Idempotency-Key` header, consistent with the PUT full-replace rule established by harden-write-paths (a replayed PUT would misrepresent intermediate state).

#### Scenario: Idempotency-Key on PUT is rejected

- **WHEN** the client supplies `Idempotency-Key` on `PUT /athlete-config`
- **THEN** the system returns `400 Bad Request` with `{"error":"idempotency_unsupported_for_put"}`
- **AND** no config row is created or modified

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
than erroring (see the `race-pacing-plan` spec). Beyond those consumptions, the
config SHALL remain otherwise-unconsumed: it does NOT relate the workouts
capability's stored `secs_in_zone_*` to these zone boundaries, and does NOT feed
any value into the race-fueling/raceprep intensity or carb-load math. Those
remaining consumptions are explicit follow-ups outside this change.

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

#### Scenario: Config is not merged into summary totals

- **WHEN** any config field is set
- **AND** the client calls `GET /summary/daily`
- **THEN** no `athlete_config` field appears in the summary `totals` (unit isolation preserved)

### Requirement: Config float values are rounded at the response boundary

The system SHALL round every numeric config value to one decimal place in HTTP responses, applying `numfmt.Round1` only at the response-building boundary; storage stays at full precision.

#### Scenario: Threshold pace rounds on read

- **WHEN** the stored `threshold_pace_sec_per_km` is `258.04999`
- **THEN** `GET /athlete-config` returns `"threshold_pace_sec_per_km": 258.0`
- **AND** the stored column is unchanged

### Requirement: athlete_config_update MCP tool mirrors PUT /athlete-config

The agent-tool registry SHALL expose an `athlete_config_update` tool that issues `PUT /athlete-config` with a payload mirroring the full REST body. Following the `set_goals` PUT precedent, the tool SHALL NOT attach an idempotency key (the endpoint rejects `Idempotency-Key` on PUT), and its description SHALL state the full-replace semantics (an omitted field is cleared) and the resulting retry-unsafety. The tool SHALL be tiered `write-confirm`, so the chat surface pauses for human confirmation before rewriting physiology config that downstream workout-target resolution consumes. The MCP announced-schema golden SHALL be regenerated additively.

#### Scenario: Agent updates the athlete config over MCP

- **WHEN** the agent calls `athlete_config_update` with a full config payload
- **THEN** the tool issues `PUT /athlete-config` with that body and no `Idempotency-Key` header
- **AND** the REST response body is returned verbatim as the tool result

#### Scenario: Chat surface pauses for confirmation

- **WHEN** the chat loop's assistant turn calls `athlete_config_update`
- **THEN** the turn pauses as a write-confirm proposal and the call is dispatched only after the user approves

#### Scenario: Announced schema stays golden

- **WHEN** the MCP announced-schema golden test runs after the tool is registered
- **THEN** `athlete_config_update` appears in `announced_schemas.json` and the golden comparison passes

### Requirement: Threshold changes append dated history snapshots

The system SHALL maintain an `athlete_config_history` table of full-row
physiology snapshots, one row per `effective_from` DATE (primary key),
carrying the same 16 nullable physiology columns as `athlete_config` (same
types and `> 0` CHECKs) plus `created_at`/`updated_at`. Whenever
`PUT /athlete-config` results in a stored state that differs from the prior
stored state on any physiology field (pointer-aware comparison across all 16
fields; timestamps excluded), the system SHALL record the new full state as
the snapshot for the current date — inserting it, or replacing an existing
same-date snapshot. A PUT that changes nothing SHALL append and modify
nothing. If a same-date replacement would make the snapshot identical to the
latest snapshot dated strictly earlier, the same-date row SHALL be removed
instead, so that no two consecutive history rows are physiology-identical.
The singleton upsert and the history maintenance SHALL be applied atomically
(a history failure fails the PUT; the two can never diverge). The singleton
remains the authoritative current read: `GET /athlete-config` and
`PUT /athlete-config` responses and error paths are unchanged by this
requirement.

#### Scenario: A changed PUT appends a snapshot

- **WHEN** the stored config has `ftp_watts: 240`
- **AND** the client calls `PUT /athlete-config` with a body whose `ftp_watts` is `255` (other fields unchanged)
- **THEN** the PUT succeeds exactly as before (200, singleton replaced)
- **AND** a new `athlete_config_history` row exists with today's `effective_from` and the full new state including `ftp_watts: 255`

#### Scenario: A no-change PUT appends nothing

- **WHEN** the daily Garmin sync re-issues `PUT /athlete-config` with a body producing a stored state identical to the current one on every physiology field
- **THEN** the PUT succeeds as before
- **AND** no history row is inserted, replaced, or deleted

#### Scenario: A second change on the same date replaces that date's snapshot

- **WHEN** a PUT earlier today changed `ftp_watts` to `255` (snapshot for today exists)
- **AND** a later PUT today changes `ftp_watts` to `260`
- **THEN** today's history row is replaced in place with the `ftp_watts: 260` state
- **AND** exactly one history row carries today's `effective_from`

#### Scenario: A same-day revert removes the day's snapshot

- **WHEN** the latest snapshot dated before today records `ftp_watts: 240`
- **AND** a PUT today changed `ftp_watts` to `255`, and a later PUT today restores the exact prior state (`ftp_watts: 240`, all other fields equal)
- **THEN** today's history row is removed
- **AND** the history is identical to before the first PUT (no two consecutive rows are physiology-identical)

#### Scenario: The first-ever PUT seeds the first snapshot

- **WHEN** no `athlete_config` row and no history rows exist
- **AND** the client calls `PUT /athlete-config` with `{"ftp_watts": 240}`
- **THEN** the singleton row is created as before
- **AND** one history row exists with today's `effective_from` and `ftp_watts: 240`

### Requirement: History read endpoint returns dated snapshots

The system SHALL expose `GET /athlete-config/history` returning the history
rows ascending by `effective_from` as
`{"history":[{"effective_from":"YYYY-MM-DD", <physiology fields>, "created_at":..., "updated_at":...}, ...]}`,
with null fields omitted (`omitempty`) and float fields rounded via
`numfmt.Round1` at the response boundary only — the same presentation rules
as `GET /athlete-config`. Optional `from` and `to` query parameters SHALL
bound `effective_from` inclusively. A malformed date parameter SHALL return
`400 Bad Request` with `{"error":"date_invalid","field":"from"|"to"}`; a
`from` later than `to` SHALL return `400 Bad Request` with
`{"error":"range_invalid"}`. There is no range cap and no pagination (history
grows only when physiology changes). An empty result — including before any
config has ever been written — SHALL return `200 OK` with `{"history":[]}`.

#### Scenario: History lists snapshots ascending

- **WHEN** history contains snapshots effective `1970-01-01` (`ftp_watts: 240`) and `2026-05-02` (`ftp_watts: 255`)
- **AND** the client calls `GET /athlete-config/history`
- **THEN** the response is `200 OK` with `history` listing both snapshots in that order
- **AND** each entry carries its `effective_from` and the full populated physiology state (nulls omitted)

#### Scenario: from/to bound the range inclusively

- **WHEN** history contains snapshots effective `1970-01-01`, `2026-05-02`, and `2026-06-10`
- **AND** the client calls `GET /athlete-config/history?from=2026-05-02&to=2026-06-10`
- **THEN** the response contains exactly the `2026-05-02` and `2026-06-10` snapshots

#### Scenario: Invalid parameters are rejected with structured errors

- **WHEN** the client calls `GET /athlete-config/history?from=not-a-date`
- **THEN** the response is `400 Bad Request` with `{"error":"date_invalid","field":"from"}`
- **AND** a call with `from=2026-06-10&to=2026-05-02` returns `400 Bad Request` with `{"error":"range_invalid"}`

#### Scenario: Empty history returns an empty list

- **WHEN** no config has ever been written (no history rows)
- **AND** the client calls `GET /athlete-config/history`
- **THEN** the response is `200 OK` with `{"history":[]}`

#### Scenario: Pace floats round at the response boundary

- **WHEN** a stored snapshot has `threshold_pace_sec_per_km` = `258.04999`
- **THEN** the history entry returns `"threshold_pace_sec_per_km": 258.0`
- **AND** the stored column is unchanged

### Requirement: History is seeded from the existing config at migration time

The migration that creates `athlete_config_history` SHALL insert one baseline
snapshot copied from the current `athlete_config` row (when one exists) with
the sentinel `effective_from = 1970-01-01`, meaning "the oldest known state —
assumed for all earlier dates"; the seed row's `created_at` records when
tracking actually began. On a database with no config row, nothing is seeded
and the first PUT creates the first snapshot. Once a config exists, history
is therefore never empty.

#### Scenario: Migration seeds one baseline snapshot

- **WHEN** the migration runs against a database whose `athlete_config` row has `ftp_watts: 240`
- **THEN** `athlete_config_history` contains exactly one row
- **AND** that row has `effective_from = 1970-01-01` and the full current physiology state including `ftp_watts: 240`

#### Scenario: Fresh database seeds nothing

- **WHEN** the migration runs against a database with no `athlete_config` row
- **THEN** `athlete_config_history` is created empty
- **AND** the first subsequent `PUT /athlete-config` creates the first snapshot

### Requirement: Thresholds are resolvable as of a date

The athlete-config service SHALL provide a service-level as-of lookup
(`ConfigAsOf(date)`) returning the physiology state — the full snapshot plus
its `effective_from` — from the latest history row with
`effective_from <= date`, or a nil result (distinct from an error) when
history is empty. Combined with the epoch-dated seed, the lookup is total for
any plausible date once a config exists. This lookup is a provider contract
for future consumers (per-sport TSS derivation, step-compliance zone
resolution, race pacing): **no existing consumer is rewired to it in this
change** — every current consumer keeps reading the singleton's current
values, and each rewiring is an explicit per-consumer follow-up.

#### Scenario: As-of resolves the state effective on a date

- **WHEN** history contains snapshots effective `1970-01-01` (`ftp_watts: 240`) and `2026-05-02` (`ftp_watts: 255`)
- **AND** the service resolves `ConfigAsOf(2026-04-15)`
- **THEN** the result is the `1970-01-01` snapshot (`ftp_watts: 240`)
- **AND** `ConfigAsOf(2026-05-02)` and `ConfigAsOf(2026-06-01)` both resolve to the `2026-05-02` snapshot (`ftp_watts: 255`)

#### Scenario: Empty history resolves to nil, not an error

- **WHEN** no config has ever been written
- **AND** the service resolves `ConfigAsOf` for any date
- **THEN** the result is nil with no error (mirroring the singleton's absent-row signal)

#### Scenario: Existing consumers still read current values

- **WHEN** history contains an older snapshot with a different `ftp_watts` than the current singleton
- **AND** a bike workout qualifying for `intensity_factor` derivation is created
- **THEN** the derivation uses the singleton's current `ftp_watts`, exactly as before this change

### Requirement: Garmin-detected thresholds are stored as an advisory singleton

The system SHALL store the latest Garmin-detected physiology in a `garmin_detected_thresholds`
singleton (the detection fields mirroring the config's physiology shapes, plus `detected_at` and
audit timestamps). `PUT /api/v1/athlete-config/garmin-detected` SHALL full-replace it and SHALL
be accepted **only from the garmin identity** (any other identity → 403 with the established
identity-guard vocabulary); per the PUT rule an `Idempotency-Key` is rejected.
`GET /api/v1/athlete-config/garmin-detected` SHALL return the latest detection (`200` with a
null body-object when none exists) to the non-garmin identities. Writing a detection SHALL NOT
read or mutate `athlete_config` and SHALL NOT create `threshold_history` rows — detections are
advisory evidence; applying one goes through the deliberate `PUT /athlete-config` flow. The
table SHALL be classified export-excluded (latest-only, re-derived by the next sync).

#### Scenario: The bridge records a detection without touching the config

- **WHEN** the garmin identity PUTs detected values including `ftp_watts: 285`
- **THEN** the detection singleton holds them with `detected_at`, and `athlete_config` and
  `threshold_history` are byte-identical to before

#### Scenario: Non-garmin identities cannot write detections

- **WHEN** the mobile or agent identity PUTs `/athlete-config/garmin-detected`
- **THEN** the response is `403` and nothing is stored

#### Scenario: No detection yet reads as null, not an error

- **WHEN** `GET /athlete-config/garmin-detected` is called before any sync has written one
- **THEN** the response is `200` with a null detection

### Requirement: The athlete-config PUT rejects the garmin identity

`PUT /api/v1/athlete-config` SHALL reject requests authenticated as the garmin identity with
`403` (established identity-guard vocabulary), so the configured physiology and its
`threshold_history` remain exclusively deliberate human/coach records — an automated writer can
never overwrite a confirmed value or clear fields Garmin does not expose.

#### Scenario: A garmin-identity config write is refused

- **WHEN** the garmin identity PUTs `/athlete-config`
- **THEN** the response is `403`, the config is unchanged, and no history snapshot is written

#### Scenario: Deliberate writes are unaffected

- **WHEN** the agent identity PUTs `/athlete-config` with confirmed values
- **THEN** the existing full-replace + snapshot-on-change behavior applies unchanged

### Requirement: A per-field source selector chooses between configured and detected values

The system SHALL persist a threshold source policy as `garmin_sourced_fields` on the config row
(default empty = all manual), whitelisted to `ftp_watts`, `lactate_threshold_hr`, `max_hr`,
`threshold_pace_sec_per_km`, and the zone groups `hr_zones` / `power_zones` (zones flip as
whole sets). `PUT /api/v1/athlete-config/sources` (non-garmin identities; garmin → 403) SHALL
full-replace the list, rejecting unknown tokens with `400 source_field_invalid`; it SHALL mutate
only the policy — never the physiology values — and the config PUT's full-replace SHALL NOT
touch the policy column. `GET /athlete-config` SHALL echo the active `sources`. A
`set_threshold_sources` MCP tool (write tier, one PUT) SHALL mirror the endpoint; its
description SHALL note that flipping a source changes effective thresholds and point at
`recompute-tss` when derived values matter.

#### Scenario: The coach flips FTP to Garmin

- **WHEN** the agent PUTs `/athlete-config/sources` with `["ftp_watts"]`
- **THEN** the policy holds exactly that list, the configured `ftp_watts` value is unchanged,
  and no threshold-history snapshot is written

#### Scenario: An unknown field token is rejected

- **WHEN** the body carries `["ftp_watts", "vo2max"]`
- **THEN** the response is `400` with `source_field_invalid` and the policy is unchanged

#### Scenario: Confirming values never resets policy

- **WHEN** `PUT /athlete-config` full-replaces the physiology while `["ftp_watts"]` is sourced
- **THEN** the policy still holds `["ftp_watts"]`

### Requirement: Computations consume the effective config

The system SHALL resolve an **effective config**: per field, the latest detection's value where
the field's source is `garmin` AND the detection carries a non-null value (manual fallback
otherwise — a garmin-sourced field with no detection never yields a hole), the confirmed value
for all other fields. `GET /api/v1/athlete-config/effective` SHALL return the resolved view with
a per-field `source` annotation. Computational consumers of athlete physiology — per-sport TSS
derivation, zone-reference target resolution, race pacing, and step compliance — SHALL consume
the effective view (wired once at the server trunk), while `GET /athlete-config` continues to
return exactly the confirmed values. With an empty policy the effective view SHALL equal the
confirmed config (shipping this change alters no computed number until a source is flipped).

#### Scenario: A garmin-sourced FTP drives TSS derivation

- **WHEN** `ftp_watts` is garmin-sourced, the detection holds 285, and the config holds 278
- **THEN** the effective view reports `ftp_watts: 285` annotated `source: "garmin"` and a new
  bike workout's power TSS derives against 285

#### Scenario: Missing detection falls back to manual

- **WHEN** `max_hr` is garmin-sourced but the latest detection carries no max HR
- **THEN** the effective `max_hr` is the confirmed value, annotated `source: "manual"`

#### Scenario: All-manual policy is behavior-identical to today

- **WHEN** `garmin_sourced_fields` is empty
- **THEN** the effective view equals the confirmed config field-for-field

