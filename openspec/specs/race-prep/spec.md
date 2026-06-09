# race-prep Specification

## Purpose

Deterministic computation primitives for race-week nutrition planning. Starts with carb-load; can grow as other "agent should not hallucinate this" primitives surface (e.g. recovery-window macros, fuelling-rate during long efforts).

## Requirements

### Requirement: Carb-load planning endpoint

The system SHALL expose `GET /race-prep/carb-load` returning a deterministic, stateless carb-loading schedule for a single race. The endpoint takes a race date, a body weight, and optional protocol parameters; it returns one schedule entry per day in `[race_date - days_before, race_date]` containing the target carbohydrate grams for that day. The endpoint performs no persistence and reads no user state — given the same inputs it always returns the same output.

#### Scenario: Default parameters produce a 4-entry schedule

- **WHEN** the client calls `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`
- **THEN** the response is `200 OK` with body shape `{race_date, body_weight_kg, params, schedule}`
- **AND** `schedule` contains exactly 4 entries: 3 carb-load days (`2026-07-21`, `2026-07-22`, `2026-07-23`) and race day (`2026-07-24`)
- **AND** entries are ordered by date ascending
- **AND** `params` echoes the effective inputs `{days_before: 3, carbs_per_kg_per_day: 10, race_day_carbs_per_kg: 2}`

#### Scenario: Each entry has the documented shape

- **WHEN** the client inspects any schedule entry
- **THEN** the entry has fields `{date, days_before, target_carbs_g, rationale}`
- **AND** `date` is a `YYYY-MM-DD` string
- **AND** `days_before` is an integer counting back from race day (`0` for race day, `3` for three days before)
- **AND** `target_carbs_g` is a positive number, rounded to 1 decimal place
- **AND** `rationale` is a human-readable label (e.g. `"carb-load day 1"`, `"race morning, pre-race meal ~3-4h before start"`)

#### Scenario: Load-day target is body_weight × carbs_per_kg_per_day

- **WHEN** the client calls with `body_weight_kg=70` and the defaults (`carbs_per_kg_per_day=10`)
- **THEN** every load-day entry's `target_carbs_g` is `700.0` (70 × 10, rounded to 1dp)

#### Scenario: Race-day target is body_weight × race_day_carbs_per_kg

- **WHEN** the client calls with `body_weight_kg=70` and the defaults (`race_day_carbs_per_kg=2`)
- **THEN** the entry with `days_before: 0` has `target_carbs_g: 140.0` (70 × 2)

#### Scenario: Custom parameters override defaults

- **WHEN** the client calls `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=80&days_before=4&carbs_per_kg_per_day=12&race_day_carbs_per_kg=3`
- **THEN** the schedule has 5 entries (4 load days + race day)
- **AND** every load-day `target_carbs_g` is `960.0` (80 × 12)
- **AND** the race-day `target_carbs_g` is `240.0` (80 × 3)
- **AND** `params` echoes `{days_before: 4, carbs_per_kg_per_day: 12, race_day_carbs_per_kg: 3}`

#### Scenario: days_before=0 returns race day only

- **WHEN** the client calls with `days_before=0`
- **THEN** the schedule contains exactly 1 entry — the race-day entry

#### Scenario: days_before=7 is accepted (upper bound inclusive)

- **WHEN** the client calls with `days_before=7`
- **THEN** the schedule contains exactly 8 entries (7 load days + race day)

#### Scenario: race_day_carbs_per_kg=0 produces a race-day entry with target 0

- **WHEN** the client calls with `race_day_carbs_per_kg=0`
- **THEN** the race-day entry exists in the schedule
- **AND** its `target_carbs_g` is `0.0`
- **AND** its `rationale` reflects "race morning"

#### Scenario: Missing race_date is rejected

- **WHEN** the client omits `race_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_required"}`

#### Scenario: Missing body_weight_kg is rejected

- **WHEN** the client omits `body_weight_kg`
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_required"}`

#### Scenario: race_date in the past is rejected

- **WHEN** the request supplies a `race_date` strictly before today (in the configured user timezone)
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_in_past"}`

#### Scenario: race_date today is accepted

- **WHEN** the request supplies a `race_date` equal to today (in the configured user timezone)
- **THEN** the response is `200 OK`
- **AND** the schedule's race-day entry uses today's date

#### Scenario: Malformed race_date is rejected

- **WHEN** `race_date` is not in `YYYY-MM-DD` format
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_invalid"}`

#### Scenario: body_weight_kg out of range is rejected

- **WHEN** `body_weight_kg` is outside `[30, 200]` (e.g. `25` or `250`)
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`

#### Scenario: days_before out of range is rejected

- **WHEN** `days_before` is outside `[0, 7]` (e.g. `-1` or `8`)
- **THEN** the system returns `400 Bad Request` with `{"error":"days_before_invalid","range":{"min":0,"max":7}}`

#### Scenario: carbs_per_kg_per_day out of range is rejected

- **WHEN** `carbs_per_kg_per_day` is outside `[1, 20]` (e.g. `0.5` or `25`)
- **THEN** the system returns `400 Bad Request` with `{"error":"carbs_per_kg_per_day_invalid","range":{"min":1,"max":20}}`

#### Scenario: race_day_carbs_per_kg out of range is rejected

- **WHEN** `race_day_carbs_per_kg` is outside `[0, 10]`
- **THEN** the system returns `400 Bad Request` with `{"error":"race_day_carbs_per_kg_invalid","range":{"min":0,"max":10}}`

#### Scenario: Non-numeric numeric params are rejected

- **WHEN** any numeric param (e.g. `body_weight_kg=heavy`) cannot be parsed as a number
- **THEN** the system returns `400 Bad Request` with `{"error":"<param>_invalid"}` for the offending param

#### Scenario: Endpoint requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)

#### Scenario: Endpoint is stateless and idempotent

- **WHEN** the client makes two identical `GET /race-prep/carb-load` requests
- **THEN** both responses are byte-for-byte identical
- **AND** no row is inserted into any database table
- **AND** the endpoint does NOT require an `Idempotency-Key` header (read-only)

### Requirement: Carb-load apply endpoint persists schedule into goal overrides

The system SHALL expose `POST /race-prep/carb-load/apply` taking the same parameters as `GET /race-prep/carb-load` (`race_date`, `body_weight_kg`, optional `days_before`, `carbs_per_kg_per_day`, `race_day_carbs_per_kg`). The endpoint computes the carb-load schedule via the same primitive as the GET endpoint, then writes the per-day carbohydrate target into the corresponding per-date goal override row. All writes happen inside a single database transaction — if any per-date write fails, the whole apply rolls back and zero overrides are persisted. The endpoint returns the schedule alongside a per-date `applied` array reporting whether each target row was newly created or merged into an existing override.

The apply step writes ONLY the `carbs_g` bound (as `{min: target_carbs_g}`, min-only — matching the existing pattern for `fiber_g` / `iron_mg`). Non-carbohydrate fields on a pre-existing override row (`kcal`, `protein_g`, etc.) are preserved verbatim; the apply step never clears or overwrites them.

#### Scenario: Apply on empty overrides creates one row per schedule day

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}` and no pre-existing override rows on the target dates
- **THEN** the response is `200 OK` with body shape `{race_date, body_weight_kg, params, schedule, applied}`
- **AND** `schedule` matches what `GET /race-prep/carb-load` would return for the same inputs (4 entries, default protocol parameters)
- **AND** `applied` contains exactly 4 entries — one per schedule day
- **AND** each `applied` entry has `{date, carbs_g_min, created: true}`
- **AND** four new rows exist in `daily_goal_overrides`, each with only the `carbs_g_min` bound populated (every other column null)
- **AND** `applied[0].carbs_g_min == 700.0` for a 70kg athlete on the default 10 g/kg load-day protocol
- **AND** the race-day entry has `carbs_g_min == 140.0` (70 × 2)

#### Scenario: Apply merges into existing override, preserving non-carb fields

- **WHEN** an override already exists on `2026-07-22` with `{kcal: {min: 2090, max: 2310}, protein_g: {min: 150, max: 190}}` and no `carbs_g` bound
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the `2026-07-22` override row now has `carbs_g: {min: 700}` AND the existing `kcal` and `protein_g` bounds unchanged
- **AND** the response's `applied` entry for `2026-07-22` has `{date: "2026-07-22", carbs_g_min: 700.0, created: false}`
- **AND** the `applied` entries for dates without prior overrides have `created: true`

#### Scenario: Apply replaces an existing carbs_g bound

- **WHEN** an override on `2026-07-22` already has `{carbs_g: {min: 500, max: 600}, kcal: {min: 2200}}`
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the row's `carbs_g` is replaced with `{min: 700}` (the new target; no max — the apply step writes min-only)
- **AND** `kcal` is preserved verbatim
- **AND** the response's `applied` entry has `created: false`

#### Scenario: Apply is atomic — partial failure rolls back

- **WHEN** the schedule has 4 target dates and the third per-date write fails (e.g. a constraint violation forced for tests)
- **THEN** the transaction rolls back
- **AND** zero rows have been written or modified in `daily_goal_overrides`
- **AND** the response is `500 Internal Server Error` (or the propagated error code from the failing write)
- **AND** the response does NOT include an `applied` array (or includes an empty array; either is acceptable as long as no partial state is implied)

#### Scenario: Apply uses POST semantics, accepts Idempotency-Key

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with an `Idempotency-Key` header
- **THEN** the existing idempotency middleware applies (deduping replays of the same key+body, consistent with every other POST write)
- **AND** the endpoint is NOT exempted from idempotency

#### Scenario: Apply rejects same param errors as GET

- **WHEN** the client calls apply with a `race_date` in the past
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_in_past"}` (same validation as the GET endpoint)
- **AND** no transaction is opened; no overrides are touched

- **WHEN** the client calls apply with `body_weight_kg=25`
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`

- **WHEN** the client calls apply with `days_before=8`
- **THEN** the system returns `400 Bad Request` with `{"error":"days_before_invalid","range":{"min":0,"max":7}}`

#### Scenario: Apply requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)
- **AND** no overrides are touched

#### Scenario: Apply round-trip is visible to /summary/daily

- **WHEN** the client calls `POST /race-prep/carb-load/apply` for `race_date=2026-07-24` (3 load days)
- **AND** the client then calls `GET /summary/daily?date=2026-07-22`
- **THEN** the response includes an `adherence.carbs_g` entry with `target: {min: 700}`
- **AND** `goal_source: "override"` (the apply produced an override on that date)

#### Scenario: Apply response order matches schedule order

- **WHEN** the client calls apply with default params (4 entries)
- **THEN** `applied[i].date == schedule[i].date` for every index
- **AND** both arrays are ordered by date ascending

### Requirement: recommend-workout-fuel endpoint

The system SHALL expose `GET /race-prep/recommend-workout-fuel` returning a stateless fueling recommendation (pre-workout, intra-workout, post-workout) for a single training session. The endpoint accepts EITHER `workout_id=<uuid>` (the parameters are derived from the row) OR the explicit triplet `sport=<enum>&duration_min=<int>&intensity_zone=<int>`, plus an optional `body_weight_kg=<float>` override. The endpoint is read-only and SHALL NOT consume an `Idempotency-Key` header.

#### Scenario: Workout-mode for a 90-minute Z3 bike with stored body weight

- **WHEN** the client calls `GET /race-prep/recommend-workout-fuel?workout_id=<id>` and the workout row has `sport: "bike"`, `started_at` and `ended_at` 90 minutes apart, and a `tss` corresponding to an intensity factor around 0.8 (Z3)
- **AND** the user has body-weight entries that resolve to `72 kg` via the rolling-7d rule
- **THEN** the response shape is:
  ```
  {
    "inputs": {
      "sport": "bike", "duration_min": 90, "intensity_zone": 3,
      "body_weight_kg": 72.0, "body_weight_source": "rolling_7d_avg",
      "workout_id": "<id>"
    },
    "pre_workout":  { "window_minutes_before": [60, 120], "carbs_g": 108, "carbs_g_per_kg": 1.5, "rationale": "..." },
    "intra_workout": { "applicable": true, "carbs_g_per_hour": 60, "carbs_g_total": 90,
                       "fluid_ml_per_hour": 600, "sodium_mg_per_hour": 500, "rationale": "..." },
    "post_workout": { "window_minutes_after": [0, 60], "carbs_g": 72, "protein_g": 21.6, "rationale": "..." },
    "notes": ["..."]
  }
  ```
- **AND** every numeric field is rounded to one decimal place at the response boundary

#### Scenario: Explicit-mode for a planned session

- **WHEN** the client calls `GET /race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3&body_weight_kg=72`
- **THEN** the response matches the workout-mode shape EXCEPT `inputs.body_weight_source` is `"explicit"` and `inputs.workout_id` is omitted

#### Scenario: Default tz is not required

- **WHEN** the client omits any timezone parameter
- **THEN** the response is unchanged — body-weight resolution operates on UTC dates (consistent with `plan_carb_load`); calendar-day arithmetic is not in play for this endpoint

#### Scenario: Both modes supplied is a 400

- **WHEN** the client passes BOTH `workout_id` AND any of `sport` / `duration_min` / `intensity_zone`
- **THEN** the system returns `400 Bad Request` with `{"error":"input_conflict"}`

#### Scenario: Neither mode supplied is a 400

- **WHEN** the client passes none of `workout_id` / `sport` / `duration_min` / `intensity_zone`
- **THEN** the system returns `400 Bad Request` with `{"error":"input_required"}`

#### Scenario: Workout id not found is a 404

- **WHEN** the client passes a `workout_id` that does not match any row
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: Partial explicit input is a 400

- **WHEN** explicit-mode is detected (at least one of `sport` / `duration_min` / `intensity_zone` is present) but any required component is missing
- **THEN** the system returns `400 Bad Request` with `{"error":"sport_required"}` / `{"error":"duration_min_required"}` / `{"error":"intensity_zone_required"}` as appropriate (first-missing-wins)

#### Scenario: Invalid sport / zone / duration values are 400s

- **WHEN** `sport` is not one of `bike` / `run` / `swim` / `row` / `strength` / `other`
- **THEN** the system returns `400 Bad Request` with `{"error":"sport_invalid"}`
- **WHEN** `intensity_zone` is outside `[1, 5]`
- **THEN** the system returns `400 Bad Request` with `{"error":"intensity_zone_invalid","range":{"min":1,"max":5}}`
- **WHEN** `duration_min` is `<= 0` or unparseable
- **THEN** the system returns `400 Bad Request` with `{"error":"duration_min_invalid"}`

#### Scenario: Invalid body_weight_kg is a 400

- **WHEN** `body_weight_kg` is supplied but is `<= 0` or NaN/Inf
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid"}`

#### Scenario: No stored weight and no override is a 400

- **WHEN** body-weight resolution fails (no override, no in-window entries, no last-before entry)
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_data_missing"}`

### Requirement: Workout-mode intensity is derived from TSS when available

The system SHALL derive `intensity_zone` from the workout's `tss` and `duration_min` using the intensity factor `IF = sqrt(tss / (duration_min/60) / 100)` mapped to bands (`IF < 0.65` → 1, `0.65-0.75` → 2, `0.75-0.85` → 3, `0.85-0.92` → 4, `> 0.92` → 5). When `tss` is absent, the system SHALL default `intensity_zone` to `2` and include a note in `pre_workout.rationale` (or a top-level `notes[]` entry) flagging the default.

#### Scenario: TSS present → derived zone

- **WHEN** the workout has `tss: 70` and `duration_min: 60` (intensity factor `sqrt(70/100) ≈ 0.84`)
- **THEN** the response's `inputs.intensity_zone` is `3`

#### Scenario: TSS absent → default Z2 with disclosure

- **WHEN** the workout has `tss: null`
- **THEN** the response's `inputs.intensity_zone` is `2`
- **AND** the response includes a note explaining that intensity was defaulted because TSS was absent

### Requirement: Pre-workout recommendation scales with sport, zone, and body weight

The system SHALL compute `pre_workout.carbs_g = pre_workout.carbs_g_per_kg × body_weight_kg`, where `carbs_g_per_kg` is selected by the table:

```
zone 1-2 (any sport ≠ strength)  → 1.0 g/kg, window [60, 120] min
zone 3                            → 1.5 g/kg, window [60, 120] min
zone 4                            → 2.0 g/kg, window [60, 180] min
zone 5                            → 1.0 g/kg, window [60, 90]  min
sport = strength                  → 0.5 g/kg, window [30, 90]  min
```

`window_minutes_before` is an inclusive `[lo, hi]` array. The `rationale` field SHALL explain the bucket choice.

#### Scenario: Zone 3 bike

- **WHEN** `sport=bike`, `duration_min=90`, `intensity_zone=3`, `body_weight_kg=72`
- **THEN** `pre_workout.carbs_g_per_kg` is `1.5`
- **AND** `pre_workout.carbs_g` is `108.0`
- **AND** `pre_workout.window_minutes_before` is `[60, 120]`

#### Scenario: Strength override

- **WHEN** `sport=strength`, `duration_min=60`, `intensity_zone=4`, `body_weight_kg=72`
- **THEN** `pre_workout.carbs_g_per_kg` is `0.5` (strength rule overrides the zone-4 rule)
- **AND** `pre_workout.window_minutes_before` is `[30, 90]`

### Requirement: Intra-workout recommendation scales with duration, zone, and sport

The system SHALL compute `intra_workout` per the table:

```
duration_min < 45  OR  sport=strength
    → applicable: false (all numeric fields nil)
sport=swim AND duration_min ≤ 120
    → applicable: false
duration 45-90 min, Zone 1-2
    → 30 g/hr CHO; fluid 500 ml/hr; sodium 300 mg/hr
duration 45-90 min, Zone 3-4
    → 60 g/hr CHO; fluid 600 ml/hr; sodium 500 mg/hr
duration 90-180 min, Zone 1-2
    → 60 g/hr CHO; fluid 600 ml/hr; sodium 450 mg/hr
duration 90-180 min, Zone 3-4
    → 60 g/hr CHO; fluid 700 ml/hr; sodium 600 mg/hr
duration > 180 min, any zone
    → 90 g/hr CHO; fluid 700 ml/hr; sodium 700 mg/hr
sport = run, any bucket where CHO would be 90
    → cap at 60 g/hr CHO
```

When `applicable: true`, `intra_workout.carbs_g_total = round1(carbs_g_per_hour × duration_min / 60)`.

#### Scenario: Sub-45-min session is not-applicable

- **WHEN** `duration_min=40`, `intensity_zone=3`, `sport=bike`
- **THEN** `intra_workout.applicable` is `false`
- **AND** all numeric fields under `intra_workout` are `null`

#### Scenario: Swim ≤ 120 min is not-applicable

- **WHEN** `sport=swim`, `duration_min=90`, `intensity_zone=3`
- **THEN** `intra_workout.applicable` is `false`

#### Scenario: 90-min Z3 bike gets 60 g/hr with computed total

- **WHEN** `sport=bike`, `duration_min=90`, `intensity_zone=3`
- **THEN** `intra_workout.carbs_g_per_hour` is `60.0`
- **AND** `intra_workout.carbs_g_total` is `90.0`

#### Scenario: 240-min Z2 run caps at 60 g/hr (sport modifier)

- **WHEN** `sport=run`, `duration_min=240`, `intensity_zone=2`
- **THEN** `intra_workout.carbs_g_per_hour` is `60.0` (capped at the run GI ceiling, NOT the 90 g/hr the duration bucket would yield for bike/row)
- **AND** the rationale notes the run-specific cap

#### Scenario: 240-min Z2 bike returns 90 g/hr

- **WHEN** `sport=bike`, `duration_min=240`, `intensity_zone=2`
- **THEN** `intra_workout.carbs_g_per_hour` is `90.0` (no cap)

### Requirement: Post-workout recommendation reuses the MPS threshold

The system SHALL compute `post_workout.carbs_g = 1.0 × body_weight_kg` and `post_workout.protein_g = 0.3 × body_weight_kg`, with `window_minutes_after = [0, 60]`. The `0.3 g/kg` factor is the same MPS threshold used by `protein_distribution` (per the meals capability "MPS threshold computation" requirement) — a single literature constant, two endpoints.

#### Scenario: 72 kg athlete post-workout

- **WHEN** `body_weight_kg=72`
- **THEN** `post_workout.carbs_g` is `72.0`
- **AND** `post_workout.protein_g` is `21.6`
- **AND** `post_workout.window_minutes_after` is `[0, 60]`

### Requirement: Notes carry the wider literature context

The response SHALL include a `notes[]` array carrying the literature-band context that cannot be encoded as single numbers — at minimum:

- the validated sodium range (300–800 mg/hr) with the personalize-to-sweat-rate caveat;
- the duration-bucket CHO/hr rule (< 45 min none, 45–90 min 30 g/hr, 90–180 min 60 g/hr, > 180 min 90 g/hr);
- a pointer to `plan_carb_load` for races > 90 min where 24–72h pre-loading matters.

#### Scenario: Notes always present

- **WHEN** the response is `200 OK`
- **THEN** `notes` is a non-empty array of strings
- **AND** at least one note mentions sodium personalization and at least one mentions `plan_carb_load` for race-week loading

### Requirement: Numeric outputs rounded at the response boundary

The system SHALL round every numeric field in the response (`pre_workout.carbs_g`, `pre_workout.carbs_g_per_kg`, `intra_workout.carbs_g_per_hour`, `intra_workout.carbs_g_total`, `intra_workout.fluid_ml_per_hour`, `intra_workout.sodium_mg_per_hour`, `post_workout.carbs_g`, `post_workout.protein_g`, `inputs.body_weight_kg`) to one decimal place. `duration_min`, `intensity_zone`, and `window_minutes_*` arrays remain integer.

#### Scenario: Body weight 72.5 yields rounded threshold-style outputs

- **WHEN** `body_weight_kg=72.5`, `sport=bike`, `duration_min=90`, `intensity_zone=3`
- **THEN** `post_workout.protein_g` is `21.8` (0.3 × 72.5 = 21.75 rounded half-away-from-zero to one decimal place)
- **AND** `post_workout.carbs_g` is `72.5`
