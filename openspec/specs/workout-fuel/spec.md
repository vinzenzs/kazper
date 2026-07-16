# workout-fuel Specification

## Purpose

Define a persisted log of in-session fueling events — gels, electrolyte drinks, salt tabs, caffeine pills, pre-race espresso — captured in their natural units (carbs in g, sodium/potassium/caffeine in mg, optional volume in ml) alongside a required free-text `name` so rehearsal data preserves WHAT was taken. Workout-fuel is the sister capability to `hydration` and `body-weight`: capture-only and deliberately unit-isolated. It is explicitly NOT an extension of `hydration_entries`, because mixing ml with grams and milligrams in a single Totals struct is the canonical footgun that the hydration capability was designed to avoid. Sodium targets during endurance work sit at 300–800 mg/hr and carb-per-hour rates dominate long-ride performance — both are invisible to a ml-only hydration log, and both belong in a structure whose schema makes its units obvious. Workout-fuel data feeds the workout-anchored fueling summary (composed in via `/workouts/{id}/fueling`), but it does NOT roll into the daily hydration or daily nutrition summaries: in-session fueling is its own protocol, distinct from food-choice macro adherence and from baseline hydration.
## Requirements
### Requirement: Workout-fuel entries are stored in a dedicated table

The system SHALL persist in-session fueling events in a `workout_fuel_entries` table independent of meals, hydration, and workouts. Each row carries a free-text `name` (required), an optional `quantity_ml`, and up to four optional measurable nutriment fields (`carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg`), plus an optional `workout_id` link, an optional `note`, and audit timestamps. At least one of `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` MUST be set per row — entries with no measurable intake are rejected.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workout_fuel_entries` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `logged_at` (TIMESTAMPTZ NOT NULL)
  - `name` (TEXT NOT NULL, CHECK `length(name) > 0`)
  - `quantity_ml` (NUMERIC(10, 1) NULL, CHECK `quantity_ml IS NULL OR quantity_ml > 0`)
  - `carbs_g` (NUMERIC(10, 1) NULL, CHECK `carbs_g IS NULL OR carbs_g >= 0`)
  - `sodium_mg` (NUMERIC(10, 1) NULL, CHECK `sodium_mg IS NULL OR sodium_mg >= 0`)
  - `potassium_mg` (NUMERIC(10, 1) NULL, CHECK `potassium_mg IS NULL OR potassium_mg >= 0`)
  - `caffeine_mg` (NUMERIC(10, 1) NULL, CHECK `caffeine_mg IS NULL OR caffeine_mg >= 0`)
  - `note` (TEXT NULL)
  - `workout_id` (UUID NULL REFERENCES workouts(id) ON DELETE SET NULL)
  - `created_at`, `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index `workout_fuel_entries_logged_at_idx` exists on `(logged_at)`
- **AND** a partial index `workout_fuel_entries_workout_id_idx` exists on `(workout_id) WHERE workout_id IS NOT NULL`

### Requirement: POST /workout-fuel logs a single entry

The system SHALL expose `POST /workout-fuel` that creates a workout-fuel entry from `{name, logged_at, quantity_ml?, carbs_g?, sodium_mg?, potassium_mg?, caffeine_mg?, note?, workout_id?}` and accepts the standard `Idempotency-Key` header.

#### Scenario: Successful log with a gel

- **WHEN** the client posts `{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"sodium_mg":0,"caffeine_mg":100}`
- **THEN** the system creates a row and returns `201 Created` with the new entry including its generated `id`

#### Scenario: Successful log with an electrolyte drink

- **WHEN** the client posts `{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","quantity_ml":500,"carbs_g":20,"sodium_mg":380}`
- **THEN** the system creates the row and returns `201 Created`
- **AND** the response includes all supplied fields

#### Scenario: Optional workout_id is accepted and validated

- **WHEN** the client posts an entry with `"workout_id": "<existing-uuid>"`
- **THEN** the entry is created with that link
- **AND** the response includes `workout_id`
- **WHEN** the client posts with `"workout_id": "<unknown-uuid>"`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** no row is created

#### Scenario: Name is required

- **WHEN** the client posts a body without `name` (or with empty string)
- **THEN** the system returns `400 Bad Request` with `{"error":"name_required"}`

#### Scenario: At least one quantitative field is required

- **WHEN** the client posts a body with `name` and `logged_at` only — all of `quantity_ml`, `carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg` are null/omitted
- **THEN** the system returns `400 Bad Request` with `{"error":"empty_entry"}`
- **AND** no row is created

#### Scenario: quantity_ml = 0 is rejected

- **WHEN** the client posts `quantity_ml: 0`
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: quantity_ml < 0 is rejected

- **WHEN** the client posts `quantity_ml: -100`
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: Nutriment fields = 0 are accepted (meaningful "explicitly zero")

- **WHEN** the client posts `caffeine_mg: 0` alongside other supplied fields
- **THEN** the entry is created and `caffeine_mg: 0` is echoed back
- **AND** this is distinct from omitting the field (which would be stored as NULL and not appear on read)

#### Scenario: Nutriment fields < 0 are rejected

- **WHEN** the client posts `sodium_mg: -5` (or any other negative nutriment)
- **THEN** the system returns `400 Bad Request` with `{"error":"<field>_invalid"}` (one of `carbs_g_invalid`, `sodium_mg_invalid`, `potassium_mg_invalid`, `caffeine_mg_invalid`)

#### Scenario: logged_at more than 24h in the future is rejected

- **WHEN** the client posts `logged_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: note longer than 500 characters is rejected

- **WHEN** `note` is longer than 500 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"note_too_long"}`

#### Scenario: Idempotent retry returns the original entry

- **WHEN** two POST requests arrive with the same `Idempotency-Key` and byte-identical body
- **THEN** the second response replays the first response body and status `201`
- **AND** only one `workout_fuel_entries` row exists

### Requirement: GET /workout-fuel lists entries in a window

The system SHALL expose `GET /workout-fuel?from=<rfc3339>&to=<rfc3339>` that returns entries whose `logged_at` falls within the half-open window, ordered by `logged_at` ascending.

#### Scenario: Window filtering returns only entries in range

- **WHEN** the client calls `GET /workout-fuel?from=…&to=…`
- **THEN** only entries with `from <= logged_at < to` are returned
- **AND** entries outside the window are excluded

#### Scenario: Missing window parameters are rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted window is rejected

- **WHEN** `from >= to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Response wraps the list

- **WHEN** the request is valid
- **THEN** the response body has the shape `{"entries": [Entry, ...]}` (consistent with `/hydration` and `/weight`)

### Requirement: PATCH /workout-fuel/{id} updates a subset of fields

The system SHALL expose `PATCH /workout-fuel/{id}` accepting partial updates of any column except `id`, `created_at`, `updated_at`. `workout_id` supports the empty-string clear semantic established by `add-meal-workout-link`. Validation rules match the POST endpoint, including the at-least-one-quantitative-field requirement if the patch would result in an empty row.

#### Scenario: Partial update changes only supplied fields

- **WHEN** the client patches `{"sodium_mg": 420}` on an existing entry
- **THEN** the response shows the new sodium value
- **AND** all other fields remain unchanged

#### Scenario: PATCH workout_id supports set / clear / no-touch

- **WHEN** the client patches `{"workout_id":"<uuid>"}` for an existing workout
- **THEN** the link is set
- **WHEN** the client patches `{"workout_id":""}`
- **THEN** the link is cleared and `workout_id` is omitted from the response
- **WHEN** the client patches without the `workout_id` field
- **THEN** the previously-stored value is preserved

#### Scenario: PATCH workout_id to a non-existent UUID is rejected

- **WHEN** the client patches `{"workout_id":"<unknown-uuid>"}`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`

#### Scenario: PATCH that would leave the row with no quantitative field is rejected

- **WHEN** the client patches `{"carbs_g": null, "quantity_ml": null}` such that the resulting row has all of `quantity_ml`, `carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg` null
- **THEN** the system returns `400 Bad Request` with `{"error":"empty_entry"}`
- **AND** the row is NOT updated

#### Scenario: PATCH to an invalid value is rejected

- **WHEN** the client patches `{"carbs_g": -1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"carbs_g_invalid"}`

#### Scenario: PATCH on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_fuel_not_found"}`

### Requirement: DELETE /workout-fuel/{id} removes an entry

The system SHALL expose `DELETE /workout-fuel/{id}` that permanently removes a workout-fuel entry.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing entry
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent reads via the list endpoint do not return it

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_fuel_not_found"}`

### Requirement: Deleting a referenced workout clears workout_id on its fuel entries

The system SHALL automatically clear `workout_id` to NULL on workout-fuel rows when the referenced workout is deleted, via the `ON DELETE SET NULL` foreign key. The fuel entry itself is preserved.

#### Scenario: Workout deletion cascades to NULL on workout-fuel rows

- **WHEN** a workout-fuel entry has `workout_id = X` and the workout `X` is deleted via `DELETE /workouts/X`
- **THEN** the entry's `workout_id` becomes NULL automatically
- **AND** subsequent reads of the entry omit the `workout_id` field
- **AND** all other fields of the entry are unchanged

### Requirement: Workout-fuel is unit-isolated from hydration and nutrition summaries

The system SHALL NOT include workout-fuel data in `GET /summary/hydration/daily` or in nutrition `GET /summary/daily` / `GET /summary/range`. Workout-fuel responses SHALL NOT contain nutriment fields that don't belong to the workout-fuel shape (no kcal, no per-100g nutriments).

#### Scenario: Daily hydration summary does not include workout_fuel ml

- **WHEN** a workout-fuel entry exists for date D with `quantity_ml: 500`
- **AND** the client calls `GET /summary/hydration/daily?date=D&tz=…`
- **THEN** the response's `total_ml` does NOT include the 500
- **AND** the response body does not include any workout-fuel fields

#### Scenario: Nutrition daily summary does not include workout_fuel carbs

- **WHEN** a workout-fuel entry exists for date D with `carbs_g: 80`
- **AND** the client calls `GET /summary/daily?date=D&tz=…`
- **THEN** the response's `totals.carbs_g` does NOT include the 80
- **AND** macro adherence is computed without workout-fuel contributions

### Requirement: A workout's sweat rate is derivable from explicit weights and logged fluid

The system SHALL expose
`GET /api/v1/workouts/{id}/sweat-rate?pre_weight_kg=&post_weight_kg=&fluid_ml_override=`
computing the standard field test over a **completed** workout: fluid intake as the sum of the
workout-linked hydration entries' ml and workout-fuel entries' `quantity_ml` (itemized in the
response; a supplied `fluid_ml_override` ≥ 0 replaces the derived sum and is echoed),
`sweat_loss_ml = (pre − post) × 1000 + fluid_ml`, and `sweat_rate_ml_per_hr` over the workout's
elapsed duration. `pre_weight_kg`/`post_weight_kg` are REQUIRED positive numbers
(`400 pre_weight_invalid` / `post_weight_invalid`; `400 fluid_override_invalid` on a negative
override). A planned workout SHALL return `409 workout_not_completed`; an unknown id
`404 not_found`. A negative loss or a rate above 5000 ml/hr SHALL still return the computed
values with `warning: "implausible_result"`. Values SHALL round to 1 decimal at the boundary;
the computation SHALL persist nothing and feed no daily hydration or nutrition total.

#### Scenario: A field test computes loss and rate

- **WHEN** a 2-hour completed ride carries 1000 ml of linked fluid and
  `pre_weight_kg=71.0&post_weight_kg=69.8` is supplied
- **THEN** the response reports `sweat_loss_ml = 2200`, `sweat_rate_ml_per_hr = 1100`, and the
  fluid itemization

#### Scenario: An override replaces derived fluid

- **WHEN** `fluid_ml_override=1500` is supplied
- **THEN** the computation uses 1500 ml, and the response shows both the override and the
  derived itemization it replaced

#### Scenario: Weight gain warns instead of refusing

- **WHEN** `post_weight_kg` exceeds `pre_weight_kg` enough to make the loss negative
- **THEN** the computed values return with `warning: "implausible_result"`

#### Scenario: A planned workout is rejected

- **WHEN** the referenced workout is not completed
- **THEN** the response is `409` with `workout_not_completed`

### Requirement: The sweat rate is readable over MCP

The system SHALL expose a `sweat_rate` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/sweat-rate` and forwarding the body verbatim (`workout_id`,
`pre_weight_kg`, `post_weight_kg` required; `fluid_ml_override` optional); the description SHALL
frame the result as a field-test calculation whose quality follows the supplied weights.

#### Scenario: Agent computes a session's sweat rate in one call

- **WHEN** the agent invokes `sweat_rate` with the workout id and both weights
- **THEN** one GET is issued and the loss, rate, and itemization return verbatim

### Requirement: A planned workout's fueling plan is computed on read

The system SHALL expose `GET /api/v1/workouts/{id}/fueling-plan?carbs_per_hr=` for a **planned**
workout, computing: estimated work `kJ = planned_tss / 100 × effective FTP × 3.6` with energy
expenditure per the kJ≈kcal convention; estimated carbohydrate burn as kcal × a CHO fraction
selected by planned IF (`< 0.60 → 45 %`, `0.60–0.75 → 55 %`, `0.75–0.85 → 70 %`,
`> 0.85 → 80 %`; IF derived as `sqrt(planned_tss/100 ÷ hours)` when not present) ÷ 4 kcal/g;
and an intake prescription from the duration ladder (`< 60 min → 0`,
`60–150 min → 30–60 g/hr`, `> 150 min → 60–90 g/hr`), its upper bound clamped by the OPTIONAL
`carbs_per_hr` capacity parameter (validated > 0 and ≤ 130 → `400 carbs_per_hr_invalid`). The
response SHALL carry the echoed inputs (planned TSS, duration, IF, FTP, fraction),
`estimated_kj`, `estimated_carb_burn_g`, `per_hour_g` and `session_total_g` ranges, and
`projected_deficit_g` (burn − maximum prescribed intake), gram values to 1 decimal at the
boundary. Degradations: a non-planned workout → `409 workout_not_planned`; no planned TSS and
no duration → `200` with `reason: "plan_data_missing"`; duration without TSS → intake guidance
only with `reason: "tss_missing"`; missing effective FTP → intake guidance only with
`reason: "ftp_missing"`. Compute-on-read; persists nothing; carb values SHALL feed no daily
nutrition total.

#### Scenario: A long planned ride gets burn and prescription

- **WHEN** a planned 3-hour ride carries 180 planned TSS and effective FTP is 280 W
- **THEN** the response reports `estimated_kj ≈ 1814`, a CHO fraction from IF ≈ 0.77, the
  60–90 g/hr prescription over 3 hours, and the projected deficit

#### Scenario: Capacity clamps the prescription

- **WHEN** `carbs_per_hr=70` is supplied for a session whose ladder allows up to 90
- **THEN** `per_hour_g` tops out at 70 and the totals follow

#### Scenario: A short session prescribes nothing

- **WHEN** the planned workout is 45 minutes
- **THEN** the prescription is 0 g/hr regardless of intensity, with burn still estimated

#### Scenario: Missing FTP degrades to guidance

- **WHEN** no effective FTP is available
- **THEN** the duration-ladder prescription returns with `reason: "ftp_missing"` and no burn
  estimate

#### Scenario: A completed workout is refused

- **WHEN** the referenced workout is completed
- **THEN** the response is `409` with `workout_not_planned`

### Requirement: The fueling plan is readable over MCP

The system SHALL expose a `workout_fueling_plan` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/fueling-plan` and forwarding the body verbatim (`workout_id`
required, `carbs_per_hr` optional). The description SHALL state the division of labor — races
carry authored `race-fueling-plan`s, this computes training-day prescriptions — and that the
athlete's gut capacity comes from rehearsal experience, not from this endpoint.

#### Scenario: Agent plans tomorrow's fueling in one call

- **WHEN** the agent invokes `workout_fueling_plan` for tomorrow's planned ride
- **THEN** the tool issues one GET and returns burn, prescription, and deficit verbatim

