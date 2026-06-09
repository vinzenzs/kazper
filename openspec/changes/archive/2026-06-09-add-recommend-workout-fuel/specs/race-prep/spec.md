## ADDED Requirements

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
