# workouts — delta for add-per-sport-tss

## ADDED Requirements

### Requirement: Workouts record TSS provenance in a tss_source column

The system SHALL add a nullable `tss_source TEXT` column to `workouts` recording
how the row's `tss` value was obtained, with allowed values exactly `garmin`,
`manual`, `power`, `pace`, and `hr`. The field is **server-managed**: it is
derived from how the TSS arrived (caller-supplied vs computed, and by which
method) and is NOT accepted as an input on `POST /workouts`, `POST
/workouts/bulk` items, or `PATCH /workouts/{id}` — a `tss_source` key in a
request body is ignored. A `tss` and its `tss_source` SHALL be paired: a
database CHECK constraint enforces `(tss IS NULL) = (tss_source IS NULL)`. The
field is returned on `GET /workouts`, `GET /workouts/{id}`, and every write
response following the `omitempty` pattern. Patching `tss` to a value SHALL set
`tss_source = 'manual'`; patching `tss` to explicit JSON `null` SHALL clear both
`tss` and `tss_source` to NULL.

#### Scenario: Column, CHECK constraints, and provenance back-fill

- **WHEN** the migration adding `tss_source` is applied to a database with existing `workouts` rows
- **THEN** `workouts` carries `tss_source` (TEXT NULL, CHECK IN `('garmin','manual','power','pace','hr')`)
- **AND** every existing row with `tss IS NOT NULL` is back-filled to `tss_source = 'garmin'` when `source = 'garmin'`, else `tss_source = 'manual'`
- **AND** every existing row with `tss IS NULL` keeps `tss_source = NULL`
- **AND** a CHECK constraint enforces `(tss IS NULL) = (tss_source IS NULL)`

#### Scenario: Caller-supplied tss_source is ignored, not stored

- **WHEN** the client posts a workout body containing `"tss_source": "power"` alongside an explicit `tss` and `source: "manual"`
- **THEN** the stored `tss_source` is `'manual'` (derived from how the value arrived)
- **AND** the request is not rejected (the unknown-input key is ignored)

#### Scenario: tss_source follows omitempty on responses

- **WHEN** the client lists a window containing one workout with `tss` set and one with `tss` NULL
- **THEN** the first entry includes `tss_source` with its stored value
- **AND** the second entry omits both the `tss` and `tss_source` keys

#### Scenario: PATCH of tss marks the value manual

- **WHEN** a workout has a computed `tss` with `tss_source = 'pace'`
- **AND** the client patches `{"tss": 85}`
- **THEN** the row's `tss = 85` and `tss_source = 'manual'`

#### Scenario: PATCH null clears both tss and tss_source

- **WHEN** a workout has `tss` and `tss_source` set
- **AND** the client patches `{"tss": null}`
- **THEN** both columns become NULL
- **AND** subsequent GET responses omit both keys

### Requirement: Completed workouts derive TSS at ingest with a fixed precedence

The system SHALL compute a TSS for a workout at write time — on `POST /workouts`,
on each `POST /workouts/bulk` item, and on the `external_id` UPSERT update path —
whenever the workout's `status` is `completed` and the caller did not supply a
`tss`, using the first applicable method in this fixed precedence (each failed
gate falls through to the next):

1. **Explicit** — a caller-supplied `tss` is stored verbatim and no derivation
   runs; `tss_source` is `'garmin'` when the workout's `source` is `garmin`,
   else `'manual'`. This tier also covers planned targets: a `tss` supplied on
   a `status='planned'` workout is stored the same way, and derivation NEVER
   runs for planned workouts.
2. **Power** (`tss_source='power'`) — gate: `sport='bike'` and an effective
   `intensity_factor > 0` (caller-supplied, or derived from
   `normalized_power_w / ftp_watts` in the same write; the existing IF
   derivation runs first). Formula: `TSS = duration_hr × IF² × 100`.
3. **Pace** (`tss_source='pace'`) —
   - rTSS, gate: `sport='run'`, `distance_m > 0`, and
     `athlete_config.threshold_pace_sec_per_km` set and `> 0`. With
     `pace = duration_s / (distance_m/1000)` sec/km:
     `IF = threshold_pace_sec_per_km / pace`, `TSS = duration_hr × IF² × 100`.
   - sTSS, gate: `sport='swim'`, `distance_m > 0`, and
     `athlete_config.threshold_swim_pace_sec_per_100m` set and `> 0`. With
     `pace = duration_s / (distance_m/100)` sec/100m:
     `IF = threshold_swim_pace_sec_per_100m / pace`,
     `TSS = duration_hr × IF³ × 100` (cubic, swim convention).
4. **HR** (`tss_source='hr'`) — gate for ANY sport: `avg_hr > 0` and an LTHR
   available — `athlete_config.lactate_threshold_hr` when set, else
   `threshold_hr`. Formula: `IF = avg_hr / LTHR`,
   `TSS = duration_hr × IF² × 100`.
5. **None** — `tss` and `tss_source` both stay NULL and the write succeeds
   without error.

`duration_hr` is derived from the workout's `ended_at − started_at` window
(elapsed time). When any method's computed `IF` exceeds `2.5` the derivation
SHALL skip that write entirely (leave `tss` NULL) rather than store an
implausible value. Derivation SHALL fail open: an unwired or empty
athlete-config never fails the write and never raises an error. The computed
value snapshots the thresholds in effect at write time and SHALL NOT be
retroactively recomputed when athlete-config later changes; `PATCH` SHALL NOT
trigger derivation. Computed values are stored at the column's 2dp precision
and rounded with `numfmt.Round1` at the response boundary like `tss` today.

#### Scenario: Caller-supplied TSS always wins with provenance

- **WHEN** the Garmin importer posts a completed `bike` workout with `tss: 78` and `source: "garmin"`
- **THEN** the stored `tss` is `78` with `tss_source = 'garmin'` and no derivation runs
- **AND** an otherwise identical workout posted with `source: "manual"` stores `tss_source = 'manual'`

#### Scenario: Bike with power derives power TSS

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client posts a completed 2-hour `bike` workout with `normalized_power_w: 200`, no `intensity_factor`, and no `tss`
- **THEN** the derived `intensity_factor` is `0.80` (existing rule)
- **AND** the stored `tss` is `128` (2 × 0.80² × 100) with `tss_source = 'power'`

#### Scenario: Run without power derives rTSS from threshold pace

- **WHEN** `athlete_config.threshold_pace_sec_per_km` is `270` (4:30/km)
- **AND** the client posts a completed 1-hour `run` workout with `distance_m: 12000` (pace 300 sec/km) and no `tss`
- **THEN** the stored `tss` is `81` (1 × (270/300)² × 100, at 2dp) with `tss_source = 'pace'`

#### Scenario: Swim derives sTSS with the cubic exponent

- **WHEN** `athlete_config.threshold_swim_pace_sec_per_100m` is `90`
- **AND** the client posts a completed 1-hour `swim` workout with `distance_m: 3600` (pace 100 sec/100m) and no `tss`
- **THEN** the stored `tss` is `72.9` (1 × (90/100)³ × 100) with `tss_source = 'pace'`

#### Scenario: hrTSS is the last-resort fallback for any sport

- **WHEN** `athlete_config.lactate_threshold_hr` is `170`
- **AND** the client posts a completed 1-hour `run` workout with `avg_hr: 153`, NO `distance_m`, and no `tss`
- **THEN** the stored `tss` is `81` (1 × (153/170)² × 100, at 2dp) with `tss_source = 'hr'`
- **AND** a completed `strength` workout with `avg_hr` set derives the same way (hrTSS is sport-agnostic)

#### Scenario: lactate_threshold_hr is preferred over threshold_hr

- **WHEN** both `lactate_threshold_hr` and `threshold_hr` are set to different values
- **AND** an hrTSS derivation runs
- **THEN** `lactate_threshold_hr` is used as the LTHR
- **AND** when only `threshold_hr` is set, it is used instead

#### Scenario: Unset thresholds fall through and never error

- **WHEN** athlete-config has no `threshold_pace_sec_per_km` and no LTHR field set
- **AND** the client posts a completed `run` workout with `distance_m` and `avg_hr` but no `tss`
- **THEN** the workout is created with `tss` and `tss_source` both NULL
- **AND** the response is `201 Created` with no error

#### Scenario: Planned workouts never derive

- **WHEN** the client posts a `status: "planned"` run with `distance_m` set, thresholds configured, and no `tss`
- **THEN** the stored `tss` is NULL (derivation gates on `status='completed'`)
- **AND** a planned workout posted WITH `tss: 60` stores it with `tss_source = 'manual'` (or `'garmin'` per source)

#### Scenario: Implausible IF skips the derivation

- **WHEN** `athlete_config.threshold_pace_sec_per_km` is `270`
- **AND** the client posts a completed `run` whose pace computes an `IF > 2.5` (e.g. a mis-tagged car ride)
- **THEN** the stored `tss` is NULL and no error is raised

#### Scenario: Re-sync re-applies the precedence in full

- **WHEN** a Garmin activity was first ingested without a `tss` and derived `tss_source = 'pace'`
- **AND** the importer re-POSTs the same `external_id` with an explicit `tss: 95`
- **THEN** the row's `tss` becomes `95` with `tss_source = 'garmin'` (full-replace re-evaluates the precedence)

#### Scenario: Bulk items derive independently

- **WHEN** a `POST /workouts/bulk` batch contains a bike with NP, a run with distance, and a swim with neither distance nor HR
- **THEN** each item derives (or not) per its own gates — `power`, `pace`, and NULL respectively — with partial-failure semantics unchanged

### Requirement: POST /workouts/recompute-tss backfills computed TSS

The system SHALL expose `POST /workouts/recompute-tss` (authenticated) that
re-runs the ingest-time TSS derivation over all `completed` workouts whose `tss`
is NULL **or** whose `tss_source` is one of the computed values
(`power`, `pace`, `hr`), against the thresholds currently in athlete-config.
Rows with `tss_source` `'garmin'` or `'manual'` SHALL never be touched. For each
candidate the derivation MAY fill a previously-NULL `tss`, update a computed
value, or clear a computed value back to NULL when no method applies anymore.
The response SHALL report `{"examined": <n>, "updated": <n>, "by_source":
{"power": <n>, "pace": <n>, "hr": <n>, "none": <n>}}` where `by_source` counts
the updated rows by their new provenance (`none` = cleared to NULL). The
endpoint is a mutating POST and participates in the idempotency middleware like
other POSTs. It SHALL be exposed as an MCP tool issuing exactly one HTTP call.

#### Scenario: Recompute fills historical NULL-TSS rows

- **WHEN** completed run and swim rows exist with `tss` NULL and the thresholds are configured
- **AND** the client calls `POST /workouts/recompute-tss`
- **THEN** those rows gain a computed `tss` with `tss_source` `'pace'` (or `'hr'` per their gates)
- **AND** the response counts them under `updated` and the matching `by_source` keys

#### Scenario: Measured values are immutable to recompute

- **WHEN** rows exist with `tss_source = 'garmin'` and `tss_source = 'manual'`
- **AND** the client calls `POST /workouts/recompute-tss`
- **THEN** those rows' `tss` and `tss_source` are unchanged
- **AND** they are not counted under `updated`

#### Scenario: Recompute after a threshold change updates computed rows

- **WHEN** a row carries `tss_source = 'pace'` computed against an old threshold pace
- **AND** `athlete_config.threshold_pace_sec_per_km` has since changed
- **AND** the client calls `POST /workouts/recompute-tss`
- **THEN** that row's `tss` is recomputed against the current threshold
- **AND** a computed row whose thresholds have been cleared entirely (no applicable method left) is cleared back to `tss = NULL` / `tss_source = NULL` and counted under `by_source.none`

#### Scenario: Recompute with nothing to do is a no-op 200

- **WHEN** every completed workout already carries a measured `tss` (or no thresholds are set and no method applies)
- **AND** the client calls `POST /workouts/recompute-tss`
- **THEN** the response is `200 OK` with `updated: 0`

#### Scenario: The recompute is exposed as an MCP tool

- **WHEN** the agent calls the `recompute_workout_tss` MCP tool
- **THEN** the MCP server issues exactly one `POST /workouts/recompute-tss` request and forwards the response verbatim
- **AND** an idempotency key is auto-derived when the agent does not supply one
