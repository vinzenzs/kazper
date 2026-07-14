# workouts Specification

## Purpose

Define a persisted catalogue of training sessions with the minimum metadata nutrition tools need — sport, time window, intensity, and burn. Workouts are a standalone primitive: the backend exposes a minimal write surface (REST endpoints for create/upsert, list, get, patch, delete, and bulk upsert), while the writer (today `garmin.py`, tomorrow potentially Apple Health, Strava, or a manual REST call) lives outside the API. The shape is source-agnostic so any external importer can target it, and `external_id` provides deduplication so a Garmin-style writer can "POST every activity it sees" without tracking what is already synced. Performance analysis (laps, splits, GPS, streams) is explicitly out of scope; this capability stores only what downstream nutrition tools need to answer "what was the athlete doing in window X?".
## Requirements
### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, a `status` (`planned` or `completed`), optional intensity/burn signals, optional ingestion metrics (distance, average power, ambient temperature, estimated sweat loss), an optional session-group key linking the legs of a brick/multisport session, optional links to a single-sport `workout-template`, a `multisport-template`, and a training-plan `plan_slot` (for planned workouts originating from a plan), and audit timestamps. A planned **multisport** workout SHALL carry `sport='multisport'` and a `multisport_template_id` (and no single-sport `template_id`); `multisport` is a workout-level sport for a structured brick/triathlon row, whereas `transition` is never a workout sport (it appears only as a segment sport inside a multisport template). The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints, and that the training-plan materializer targets for planned sessions.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NULL)
  - `source` (TEXT NOT NULL, CHECK IN `('garmin', 'manual', 'other')`)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'multisport', 'other')`)
  - `status` (TEXT NOT NULL DEFAULT `'completed'`, CHECK IN `('planned', 'completed')`)
  - `name` (TEXT NULL)
  - `started_at` (TIMESTAMPTZ NOT NULL)
  - `ended_at` (TIMESTAMPTZ NOT NULL)
  - `kcal_burned` (NUMERIC(10, 1) NULL, CHECK `kcal_burned IS NULL OR kcal_burned > 0`)
  - `avg_hr` (INTEGER NULL, CHECK `avg_hr IS NULL OR avg_hr > 0`)
  - `tss` (NUMERIC(10, 2) NULL, CHECK `tss IS NULL OR tss >= 0`)
  - `rpe` (INTEGER NULL, CHECK `rpe IS NULL OR (rpe BETWEEN 1 AND 10)`)
  - `gi_distress_score` (INTEGER NULL, CHECK `gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5)`)
  - `distance_m` (NUMERIC(10, 1) NULL, CHECK `distance_m IS NULL OR distance_m > 0`)
  - `avg_power_w` (INTEGER NULL, CHECK `avg_power_w IS NULL OR avg_power_w > 0`)
  - `temperature_c` (NUMERIC(4, 1) NULL, CHECK `temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60)`)
  - `sweat_loss_ml` (NUMERIC(10, 1) NULL, CHECK `sweat_loss_ml IS NULL OR sweat_loss_ml > 0`)
  - `session_group` (TEXT NULL)
  - `template_id` (UUID NULL, REFERENCES `workout_templates(id)` ON DELETE SET NULL)
  - `multisport_template_id` (UUID NULL, REFERENCES `multisport_templates(id)` ON DELETE SET NULL)
  - `plan_slot_id` (UUID NULL, REFERENCES `plan_slots(id)` ON DELETE SET NULL)
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** a partial (non-unique) index `workouts_session_group_idx` exists on `(session_group) WHERE session_group IS NOT NULL`
- **AND** a partial UNIQUE index `workouts_plan_slot_id_key` exists on `(plan_slot_id) WHERE plan_slot_id IS NOT NULL`
- **AND** there is NO `intensity` column (TSS is the intensity signal; downstream tools derive bands at call time)

#### Scenario: A multisport planned workout row is accepted

- **WHEN** a planned workout is created with `sport='multisport'` and a
  `multisport_template_id`
- **THEN** the row persists and is returned with that sport and multisport
  template reference

#### Scenario: rpe and gi_distress_score are nullable per session

- **WHEN** the migration is applied to a database with existing `workouts` rows
- **THEN** every existing row carries `rpe = NULL` and `gi_distress_score = NULL`
- **AND** the migration succeeds without back-filling either column
- **AND** subsequent INSERT/UPSERT/PATCH paths default both fields to NULL when omitted

#### Scenario: Ingestion-metric columns are nullable with no back-fill

- **WHEN** the migration adding `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and `session_group` is applied to a database with existing `workouts` rows
- **THEN** every existing row carries NULL for all five columns
- **AND** the migration succeeds without back-filling any of them
- **AND** subsequent INSERT/UPSERT/PATCH paths default all five fields to NULL when omitted ("not measured" / "not grouped" is a meaningful state, not a data-quality bug)

#### Scenario: status defaults to completed and back-fills existing rows

- **WHEN** the migration adding `status` is applied to a database with existing `workouts` rows
- **THEN** every existing row takes `status = 'completed'` via the column DEFAULT (existing rows are all completed activities)
- **AND** a POST that omits `status` stores `'completed'`
- **AND** the `status` column is NOT NULL and always present on responses (no omitempty)

#### Scenario: sport vocabulary admits yoga and mobility

- **WHEN** the migration widening the `sport` CHECK is applied to a database with existing `workouts` rows
- **THEN** the `sport` CHECK accepts `'yoga'` and `'mobility'` in addition to `'run'`, `'bike'`, `'swim'`, `'strength'`, `'other'`
- **AND** every existing row keeps its current sport unchanged (the migration only widens the allowed set)

### Requirement: POST /workouts creates or updates a workout via external_id UPSERT

The system SHALL expose `POST /workouts` that accepts a workout body and persists it. When `external_id` is present and a row already exists with the same `external_id`, the system UPDATES that row (full-replace of the mutable fields); otherwise the system INSERTS a new row. The mutable field set includes `rpe` and `gi_distress_score` as optional integer-valued per-session signals (1..10 and 1..5 respectively), plus the optional ingestion metrics `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and the optional `session_group` key. This semantic lets an external writer "POST every activity it sees" without tracking what is already synced.

#### Scenario: First POST with external_id inserts a new row

- **WHEN** the client posts `{"external_id":"garmin:1234567","source":"garmin","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","kcal_burned":850,"tss":78}`
- **THEN** the system creates a row and returns `201 Created` with the new workout including its generated `id`

#### Scenario: Subsequent POST with same external_id updates the existing row

- **WHEN** a workout with `external_id: "garmin:1234567"` already exists
- **AND** the client posts another body with the same `external_id` but `kcal_burned: 900`
- **THEN** the system UPDATES the existing row to the new values
- **AND** returns `200 OK` with the updated workout (the same `id`)
- **AND** no duplicate row is created

#### Scenario: POST without external_id always inserts

- **WHEN** the client posts a body without an `external_id` (e.g. `{"source":"manual","sport":"strength","started_at":"…","ended_at":"…"}`)
- **THEN** the system INSERTS a new row with `external_id: NULL`
- **AND** returns `201 Created`
- **AND** two such POSTs produce two distinct rows even if the bodies are identical (manual writes have no implicit dedup)

#### Scenario: source is required and validated

- **WHEN** the client posts a body without `source`, or with `source` not in the documented enum
- **THEN** the system returns `400 Bad Request` with `{"error":"source_invalid"}`

#### Scenario: sport is required and validated

- **WHEN** the client posts a body without `sport`, or with `sport` not in `run|bike|swim|strength|yoga|mobility|other`
- **THEN** the system returns `400 Bad Request` with `{"error":"sport_invalid"}`

#### Scenario: yoga and mobility are accepted sports

- **WHEN** the client posts a body with `sport: "yoga"` or `sport: "mobility"` and an otherwise valid payload
- **THEN** the system persists the row with that sport and returns `201 Created`
- **AND** the response echoes the sport unchanged (no coercion to `other`/`strength`)

#### Scenario: started_at and ended_at are required and validated

- **WHEN** the client posts a body where `started_at` or `ended_at` is missing or unparseable as RFC 3339
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: ended_at must be after started_at

- **WHEN** the client posts a body where `ended_at <= started_at`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: started_at far in the future is rejected

- **WHEN** the client posts `started_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: kcal_burned if supplied must be positive

- **WHEN** the client posts `kcal_burned` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"kcal_burned_invalid"}`

#### Scenario: avg_hr if supplied must be positive integer

- **WHEN** the client posts `avg_hr` that is zero, negative, or non-integer
- **THEN** the system returns `400 Bad Request` with `{"error":"avg_hr_invalid"}`

#### Scenario: tss if supplied must be non-negative

- **WHEN** the client posts `tss` that is negative
- **THEN** the system returns `400 Bad Request` with `{"error":"tss_invalid"}`

#### Scenario: POST with rpe and gi_distress_score stores the values

- **WHEN** the client posts `{"source":"manual","sport":"bike","started_at":"2026-07-15T08:00:00Z","ended_at":"2026-07-15T09:30:00Z","rpe":7,"gi_distress_score":2}`
- **THEN** the system creates a row with `rpe = 7` and `gi_distress_score = 2`
- **AND** returns `201 Created` with the response body echoing both fields

#### Scenario: POST omitting rpe and gi_distress_score stores NULL

- **WHEN** the client posts a workout body that omits both fields
- **THEN** the row is created with both columns `NULL`
- **AND** the response body's JSON omits both fields (omitempty pattern matching `kcal_burned`, `avg_hr`, `tss`, `notes`)

#### Scenario: POST with rpe out of range is rejected

- **WHEN** the client posts `{"source":"manual","sport":"bike",…,"rpe":0}` or `{"…","rpe":11}` or `{"…","rpe":-1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **AND** no row is inserted

#### Scenario: POST with gi_distress_score out of range is rejected

- **WHEN** the client posts a workout body with `gi_distress_score` set to `0` or `6` or `-2` or `100`
- **THEN** the system returns `400 Bad Request` with `{"error":"gi_distress_score_invalid","range":{"min":1,"max":5}}`
- **AND** no row is inserted

#### Scenario: POST with non-integer rpe / gi_distress_score is rejected

- **WHEN** the client posts with `rpe: "seven"` or `rpe: 7.5` or `gi_distress_score: "mild"`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid"}` or `{"error":"gi_distress_score_invalid"}` respectively
- **AND** no row is inserted

#### Scenario: Garmin import path passes through with NULLs on rehearsal fields

- **WHEN** the Garmin importer POSTs a workout body that does not include `rpe` or `gi_distress_score` (Garmin does not surface either)
- **THEN** the row is created with both fields `NULL`
- **AND** the user can subsequently PATCH the row to add the rehearsal signals

#### Scenario: POST with all five ingestion metrics stores them

- **WHEN** the client posts `{"external_id":"garmin:555","source":"garmin","sport":"bike","started_at":"2026-06-13T08:00:00Z","ended_at":"2026-06-13T11:00:00Z","distance_m":80500,"avg_power_w":182,"temperature_c":27.5,"sweat_loss_ml":2400,"session_group":"garmin:554"}`
- **THEN** the system creates a row carrying all five values
- **AND** returns `201 Created` with the response body echoing all five fields
- **AND** `distance_m` and `sweat_loss_ml` are rounded to 1 decimal place at the response boundary

#### Scenario: distance_m if supplied must be positive

- **WHEN** the client posts `distance_m` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"distance_m_invalid"}`
- **AND** no row is inserted

#### Scenario: avg_power_w if supplied must be a positive integer

- **WHEN** the client posts `avg_power_w` that is zero, negative, or non-integer
- **THEN** the system returns `400 Bad Request` with `{"error":"avg_power_w_invalid"}`
- **AND** no row is inserted

#### Scenario: temperature_c if supplied must be within [-40, 60]

- **WHEN** the client posts `temperature_c` of `-41` or `61` or `98.6`
- **THEN** the system returns `400 Bad Request` with `{"error":"temperature_c_invalid","range":{"min":-40,"max":60}}`
- **AND** no row is inserted

#### Scenario: temperature_c accepts negative values in range

- **WHEN** the client posts `temperature_c: -5.5` (winter session)
- **THEN** the row is created with `temperature_c = -5.5`

#### Scenario: sweat_loss_ml if supplied must be positive

- **WHEN** the client posts `sweat_loss_ml` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"sweat_loss_ml_invalid"}`
- **AND** no row is inserted

#### Scenario: session_group must be non-empty and bounded when supplied

- **WHEN** the client posts `session_group` that is empty, whitespace-only, or longer than 255 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"session_group_invalid"}`
- **AND** no row is inserted

#### Scenario: Two legs of a brick share a session_group

- **WHEN** the importer posts a bike leg and a run leg, both with `session_group: "garmin:9876543"` (the multisport parent activity's id)
- **THEN** both rows persist with the same `session_group` value
- **AND** each row keeps its own real `sport`, time window, and metrics (no merged pseudo-workout is created)

#### Scenario: UPSERT full-replace covers the ingestion metrics

- **WHEN** a workout with `external_id: "garmin:555"` exists with `sweat_loss_ml = 2400`
- **AND** the client re-POSTs the same `external_id` with a body that omits `sweat_loss_ml`
- **THEN** the row's `sweat_loss_ml` becomes `NULL` (full-replace of the mutable field set, matching the existing UPSERT semantics)

### Requirement: GET /workouts lists workouts in a window

The system SHALL expose `GET /workouts?from=<rfc3339>&to=<rfc3339>` that returns workouts whose `started_at` falls in the inclusive window, ordered by `started_at` ascending. An optional `session_group=<key>` query parameter SHALL narrow the result to workouts whose `session_group` equals the supplied key exactly — an additional AND-predicate inside the (still mandatory) window, used to fetch the legs of one brick/multisport session together.

#### Scenario: Window filtering returns only workouts in range

- **WHEN** the client calls `GET /workouts?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z`
- **THEN** only workouts with `from <= started_at <= to` are returned
- **AND** workouts outside the window are excluded

#### Scenario: Missing window parameters are rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted window is rejected

- **WHEN** `from > to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Response wraps the list

- **WHEN** the request is valid
- **THEN** the response body has the shape `{"workouts": [Workout, ...]}` (consistent with `/meals` and `/hydration`)

#### Scenario: List includes rehearsal fields per row

- **WHEN** the client lists workouts in a window containing one rehearsal-tagged ride (with `rpe`/`gi_distress_score` set) and one Garmin-imported ride (both fields `NULL`)
- **THEN** the rehearsal-tagged ride's entry includes `rpe` and `gi_distress_score`
- **AND** the Garmin-imported ride's entry omits both fields (omitempty)

#### Scenario: session_group filter returns only matching legs

- **WHEN** a window contains a bike leg and a run leg with `session_group: "garmin:9876543"` plus an unrelated swim with `session_group = NULL`
- **AND** the client calls `GET /workouts?from=…&to=…&session_group=garmin:9876543`
- **THEN** exactly the two legs are returned, ordered by `started_at` ascending (leg order)
- **AND** the swim is excluded

#### Scenario: session_group filter still requires the window

- **WHEN** the client calls `GET /workouts?session_group=garmin:9876543` without `from`/`to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}` (the filter composes with, and does not replace, the window contract)

#### Scenario: session_group filter matching nothing returns an empty list

- **WHEN** no workout in the window carries the supplied `session_group`
- **THEN** the response is `200 OK` with `{"workouts": []}`

#### Scenario: List includes ingestion metrics per row (omitempty)

- **WHEN** the client lists a window containing one Garmin-imported ride with `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml` set and one manual gym session with all five NULL
- **THEN** the ride's entry includes the set fields
- **AND** the gym session's entry omits all five keys

### Requirement: GET /workouts/{id} returns a single workout

The system SHALL expose `GET /workouts/{id}` returning the workout row, including the scalar performance and HR-zone fields inline (when set) and the nested `splits` and `sets` detail arrays (each ordered by its index; empty arrays omitted). The response carries `rpe` and `gi_distress_score` when set on the underlying row; all detail and rehearsal fields follow the omitempty pattern when NULL/absent.

#### Scenario: Existing id returns the workout

- **WHEN** the client calls `GET /workouts/<existing-id>`
- **THEN** the response is `200 OK` with the workout body

#### Scenario: Unknown id returns 404

- **WHEN** the client calls `GET /workouts/<unknown-id>`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: GET on a workout with rpe and gi_distress_score returns them

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response is `200 OK` with body that includes `"rpe": 7` and `"gi_distress_score": 2`

#### Scenario: GET on a workout with NULL rehearsal fields omits them

- **WHEN** a workout has both fields `NULL`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body does NOT include the `rpe` or `gi_distress_score` keys

#### Scenario: GET returns scalar, zone, and nested detail when present

- **WHEN** a workout has elevation/normalized-power/zone columns set and three `workout_splits` rows
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the body includes the scalar fields, `secs_in_zone_1..5`, and a `splits` array of three entries ordered by `split_index`
- **AND** a strength workout with sets returns a `sets` array ordered by `set_index`

#### Scenario: GET omits detail keys when no detail exists

- **WHEN** a workout has no detail columns set and no split/set rows
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the scalar/zone keys are omitted and no empty `splits`/`sets` arrays appear

### Requirement: PATCH /workouts/{id} updates the mutable subset

The system SHALL expose `PATCH /workouts/{id}` accepting partial updates of `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`, `rpe`, `gi_distress_score`, `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and `session_group`. Validation rules match the POST endpoint for the same fields. The fields `source`, `external_id`, `sport`, `started_at`, and `ended_at` are IMMUTABLE via PATCH. PATCH supports tri-state semantics on the two integer rehearsal fields AND on the five ingestion fields: `unchanged` when absent from the body, `set` when present with a value, and `cleared to NULL` when present with explicit JSON `null`.

#### Scenario: Partial update changes only supplied mutable fields

- **WHEN** the client patches `{"tss":85,"notes":"FTP changed last month; updated TSS"}` on an existing workout
- **THEN** the response shows the new TSS and notes
- **AND** other fields remain unchanged

#### Scenario: Patching an immutable field is rejected

- **WHEN** the client patches a body containing any of `source`, `external_id`, `sport`, `started_at`, `ended_at`
- **THEN** the system returns `400 Bad Request` with `{"error":"field_immutable","field":"<offending-field>"}`

#### Scenario: Patching to a negative tss is rejected

- **WHEN** the client patches `{"tss":-1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"tss_invalid"}`

#### Scenario: Patch on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: PATCH sets rpe and gi_distress_score on an existing workout

- **WHEN** a workout exists with both fields `NULL`
- **AND** the client patches `{"rpe": 7, "gi_distress_score": 2}`
- **THEN** the row's `rpe = 7` and `gi_distress_score = 2`
- **AND** the response is `200 OK` with the updated workout

#### Scenario: PATCH absent rehearsal fields leaves them unchanged

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client patches `{"notes": "felt strong"}` (no rpe / no gi_distress_score)
- **THEN** the row's `rpe` and `gi_distress_score` are unchanged
- **AND** `notes` is updated to `"felt strong"`

#### Scenario: PATCH explicit null clears the rehearsal field to NULL

- **WHEN** a workout has `rpe = 7`
- **AND** the client patches `{"rpe": null}`
- **THEN** the row's `rpe` becomes `NULL`
- **AND** subsequent GET responses omit the `rpe` field

#### Scenario: PATCH rpe out of range is rejected without touching other fields

- **WHEN** the client patches `{"rpe": 11, "gi_distress_score": 3}`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **AND** no field is updated (transactional validation — the GI score is NOT written even though it's valid)

#### Scenario: PATCH sets ingestion metrics on an existing workout

- **WHEN** a workout exists with all five ingestion fields `NULL`
- **AND** the client patches `{"sweat_loss_ml": 1850, "temperature_c": 31}`
- **THEN** the row's `sweat_loss_ml = 1850` and `temperature_c = 31`
- **AND** the other three ingestion fields remain `NULL`
- **AND** the response is `200 OK` with the updated workout

#### Scenario: PATCH explicit null clears an ingestion field

- **WHEN** a workout has `session_group = "garmin:9876543"` (grouped by mistake)
- **AND** the client patches `{"session_group": null}`
- **THEN** the row's `session_group` becomes `NULL`
- **AND** subsequent GET responses omit the `session_group` field
- **AND** the same null-clears semantics apply to `distance_m`, `avg_power_w`, `temperature_c`, and `sweat_loss_ml`

#### Scenario: PATCH ingestion field validation matches POST

- **WHEN** the client patches `{"temperature_c": 98.6}` or `{"distance_m": -100}` or `{"session_group": ""}`
- **THEN** the system returns `400 Bad Request` with the corresponding error code (`temperature_c_invalid`, `distance_m_invalid`, `session_group_invalid`)
- **AND** no field is updated

### Requirement: DELETE /workouts/{id} removes the row

The system SHALL expose `DELETE /workouts/{id}` that permanently removes a workout.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing workout
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent GETs for that id return `404 workout_not_found`

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

### Requirement: POST /workouts/bulk upserts an array with per-item results

The system SHALL expose `POST /workouts/bulk` that accepts a batch of workouts and persists each one independently with the same upsert semantics as `POST /workouts`. Each batch item MAY carry optional nested `splits` and `sets` arrays; when present, the item's workout row and its child rows are written in a single transaction, and on an `external_id` match the child rows are fully REPLACED (delete-then-reinsert) so a re-sync never accumulates duplicate laps or sets. Per-item validation and persistence failures are reported per-item; the overall response is `200 OK` whenever the request body is well-formed and within the size cap. Partial failure is allowed.

#### Scenario: Mixed batch produces per-item results

- **WHEN** the client posts `{"workouts": [valid_1, valid_2_with_existing_external_id, invalid_3]}`
- **THEN** the system returns `200 OK` with body shape:
  ```
  {
    "results": [
      {"index": 0, "id": "<uuid>", "created": true},
      {"index": 1, "id": "<uuid>", "created": false},
      {"index": 2, "error": "<code>"}
    ]
  }
  ```
- **AND** the valid items are persisted (item 0 inserted, item 1 updated)
- **AND** the invalid item is NOT persisted
- **AND** later items continue processing even when an earlier item failed

#### Scenario: Each item uses the same external_id UPSERT semantics

- **WHEN** the batch contains an item with an `external_id` matching an existing row
- **THEN** that item's `results` entry has `created: false` and the existing row's `id`
- **AND** the row is updated to the batch item's values

#### Scenario: Nested splits and sets are written and replaced on re-sync

- **WHEN** a batch item carries `external_id: "garmin:1234567"` with `splits: [s0, s1, s2]` and is posted
- **THEN** the workout row is upserted and three `workout_splits` rows are written in the same transaction
- **AND** re-posting the same `external_id` with `splits: [s0', s1']` REPLACES the children, leaving exactly two split rows (the prior three deleted)
- **AND** a strength item carrying `sets: [...]` writes `workout_sets` rows under the same replace-on-resync semantics

#### Scenario: A child-write failure fails only its own item

- **WHEN** one batch item's nested split/set data is invalid
- **THEN** that item's transaction rolls back and its `results` entry carries the error
- **AND** other items in the batch are persisted normally (partial failure preserved)

#### Scenario: Empty array is rejected

- **WHEN** the client posts `{"workouts": []}`
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_empty"}`

#### Scenario: Batches larger than 100 items are rejected

- **WHEN** the `workouts` array contains more than 100 items
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_too_large","max":100}`
- **AND** NO items are persisted

#### Scenario: Missing or non-array workouts field is rejected

- **WHEN** the client posts a body without `workouts`, or with `workouts` not a JSON array
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_invalid"}`

#### Scenario: Per-item errors use the same codes as single POST

- **WHEN** one batch item has `sport: "pilates"` (a value outside the documented sport vocabulary)
- **THEN** its `results` entry is `{"index": <i>, "error": "sport_invalid"}` (matching the single-item POST error code)

### Requirement: Workouts are source-tagged but source-agnostic in shape

The system SHALL accept workouts from any external writer through the same endpoint and shape. The `source` field records provenance (`garmin`, `manual`, `other`) but does NOT affect persistence semantics, validation rules, or returned shape — a Garmin-sourced workout and a manual workout are stored in the same table with the same columns.

#### Scenario: A manual workout has the same response shape as a Garmin workout

- **WHEN** one workout is posted with `source: "manual"` and another with `source: "garmin"` (otherwise-identical fields)
- **THEN** both responses have the same JSON keys
- **AND** both appear in `GET /workouts` results equally

#### Scenario: source: "other" is accepted for unanticipated writers

- **WHEN** a future writer (e.g. Apple Health bridge) posts with `source: "other"`
- **THEN** the system accepts the workout
- **AND** the writer is responsible for any source-specific conventions outside the API

### Requirement: GET /workouts/{id}/fueling returns pre/intra/post intake windows

The system SHALL expose `GET /workouts/{id}/fueling?pre_window_min=<int>&post_window_min=<int>` returning three time-anchored buckets — pre, intra, post — each carrying **three** separate aggregations for entries whose `logged_at` falls within the corresponding window: a **nutrition** sub-object (from `meal_entries`), a **hydration** sub-object (from `hydration_entries`), and a **workout_fuel** sub-object (from `workout_fuel_entries`). The windows are derived from the workout's `started_at` and `ended_at` plus the supplied (or defaulted) pre/post minutes. Aggregation is time-window-based: any entry whose `logged_at` falls in a window is included regardless of its `workout_id` value.

#### Scenario: Default windows are 240 min pre / 60 min post

- **WHEN** the client calls `GET /workouts/{id}/fueling` without `pre_window_min` or `post_window_min`
- **THEN** `pre_window.minutes` is `240`
- **AND** `post_window.minutes` is `60`

#### Scenario: Response shape carries three separate sub-objects per window

- **WHEN** the response is well-formed
- **THEN** each window object has the shape `{start, end, minutes, nutrition: {totals, entry_count}, hydration: {total_ml, entry_count}, workout_fuel: {totals, entry_count}}`
- **AND** the `nutrition.totals` shape matches `/summary/daily.totals` (macros + nullable micros)
- **AND** the `workout_fuel.totals` shape carries `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` — each field nullable, summed across contributing entries
- **AND** units never mix: no ml inside `nutrition.totals`; no kcal inside `hydration` or `workout_fuel`; no per-100g nutriments inside `workout_fuel`

#### Scenario: Workout-fuel sub-object sums contributing entries

- **WHEN** two workout-fuel entries fall in the intra window with `{carbs_g: 25, sodium_mg: 100}` and `{carbs_g: 25, sodium_mg: 200, quantity_ml: 500}`
- **THEN** `intra_window.workout_fuel.totals` is `{carbs_g: 50, sodium_mg: 300, quantity_ml: 500}`
- **AND** `intra_window.workout_fuel.entry_count` is `2`
- **AND** `quantity_ml` is summed across only those entries that supplied it; `null + 500 = 500`, not `null`

#### Scenario: Workout-fuel sub-object is present even when there are no contributing entries

- **WHEN** no workout-fuel entries fall in a particular window
- **THEN** `workout_fuel.entry_count` is `0`
- **AND** `workout_fuel.totals` carries zeros (or nulls) for every field — the sub-object is NOT omitted

#### Scenario: Pre-window covers [started_at − pre_window_min, started_at)

- **WHEN** a meal is logged 30 minutes before the workout's `started_at`
- **AND** `pre_window_min >= 30`
- **THEN** the meal contributes to `pre_window.nutrition.totals`
- **AND** the meal does NOT contribute to `intra_window` or `post_window`

#### Scenario: Intra-window covers [started_at, ended_at)

- **WHEN** a hydration entry is logged at a time T with `started_at <= T < ended_at`
- **THEN** the entry contributes to `intra_window.hydration.total_ml`

#### Scenario: Post-window covers [ended_at, ended_at + post_window_min)

- **WHEN** a meal is logged 30 minutes after `ended_at`
- **AND** `post_window_min >= 30`
- **THEN** the meal contributes to `post_window.nutrition.totals`

#### Scenario: Boundary at started_at lands in intra_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.started_at`
- **THEN** the entry contributes to `intra_window.workout_fuel` (not `pre_window`)
- **AND** the response documents the half-open convention

#### Scenario: Boundary at ended_at lands in post_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.ended_at`
- **THEN** the entry contributes to `post_window.workout_fuel` (intra window is `[started_at, ended_at)`)

#### Scenario: Entries with workout_id but outside the time window are excluded

- **WHEN** any intake row (meal, hydration, or workout-fuel) has `workout_id = X` but is logged 8 hours before workout X's `started_at`
- **AND** `pre_window_min = 240` (4h, default)
- **THEN** the row does NOT appear in the fueling totals for any window
- **AND** the response shape is unchanged (no "tagged-but-outside" bucket)

#### Scenario: Entries without workout_id but inside the time window are included

- **WHEN** any intake row has `workout_id = NULL` but `logged_at` falls inside the pre-window
- **THEN** the row contributes to `pre_window.<sub-object>.totals` (time-window matching, not tag matching)

#### Scenario: Empty windows return zero totals and entry_count

- **WHEN** a workout has no meals, hydration, or workout-fuel in any window
- **THEN** every window returns `entry_count: 0` and zero totals across all three sub-objects
- **AND** the response status is `200 OK`

#### Scenario: Workout not found returns 404

- **WHEN** the client calls `GET /workouts/<unknown-uuid>/fueling`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: pre_window_min and post_window_min are bounded [0, 720]

- **WHEN** `pre_window_min` or `post_window_min` is outside `[0, 720]`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid","range":{"min":0,"max":720}}`

#### Scenario: pre_window_min = 0 returns an empty pre-window

- **WHEN** the client passes `pre_window_min=0`
- **THEN** `pre_window.minutes` is `0`
- **AND** every sub-object's `entry_count` is `0`
- **AND** the same applies symmetrically for `post_window_min=0`

#### Scenario: Numeric fields are rounded at the response boundary

- **WHEN** any aggregated total resolves to `419.7666…`
- **THEN** the response shows `419.8` (matching the existing nutrient-rounding rule)
- **AND** hydration `total_ml` and workout_fuel `quantity_ml` are rounded to 1 decimal place

### Requirement: GET /workouts/{id}/fueling surfaces rehearsal signals on the workout

The system SHALL include `rpe`, `gi_distress_score`, `sweat_loss_ml`, and `temperature_c` on the `GET /workouts/{id}/fueling` response so the agent can read the rehearsal-outcome signals and the sweat/heat context alongside the fueling totals in a single call. The fields are echoed at the top level of the response, alongside `workout_id`, `started_at`, `ended_at`, and follow the same omitempty rule as everywhere else — absent when NULL on the underlying workout row. `distance_m`, `avg_power_w`, and `session_group` are deliberately NOT echoed here: they are not inputs to fueling-adequacy judgment, and the capability excludes performance analysis.

#### Scenario: Fueling response carries rpe and gi_distress_score when set

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body includes `"rpe": 7` and `"gi_distress_score": 2` at the top level (alongside `workout_id`, `started_at`, `ended_at`, `pre_window`, `intra_window`, `post_window`)

#### Scenario: Fueling response omits the fields when NULL

- **WHEN** a workout has both fields `NULL`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body omits the `rpe` and `gi_distress_score` keys
- **AND** the fueling window shapes are otherwise unchanged

#### Scenario: Fueling endpoint requires no new query params for the new fields

- **WHEN** the client calls `GET /workouts/{id}/fueling`
- **THEN** the existing `pre_window_min` / `post_window_min` query semantics apply unchanged
- **AND** no `include_rehearsal` opt-in is required — the fields are always present (or always omitted via omitempty)

#### Scenario: Fueling response carries sweat_loss_ml and temperature_c when set

- **WHEN** a workout has `sweat_loss_ml = 2400` and `temperature_c = 27.5`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body includes `"sweat_loss_ml": 2400` and `"temperature_c": 27.5` at the top level
- **AND** the agent can compare `sweat_loss_ml` against the summed fluid intake (`hydration.total_ml` + `workout_fuel.totals.quantity_ml` across the windows) in one call

#### Scenario: Fueling response omits sweat/heat context when NULL and never echoes performance fields

- **WHEN** a workout has `sweat_loss_ml` and `temperature_c` `NULL` but `distance_m` and `avg_power_w` set
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body omits `sweat_loss_ml` and `temperature_c`
- **AND** the response body does NOT contain `distance_m`, `avg_power_w`, or `session_group` keys regardless of their values on the row

### Requirement: Workouts carry a planned/completed status lifecycle

The system SHALL treat `status` as a mutable workout field with values `planned` and `completed` (default `completed`). `status` conditions the future-date guard: a `completed` workout keeps the existing rule (rejected when `started_at` is more than 24 hours in the future), while a `planned` workout MAY have a `started_at` in the future up to one year ahead (beyond that, `started_at_too_far_future` still fires). A `planned` workout MAY have a past `started_at` (a plan already underway). `ended_at > started_at` holds for both. Reconciling a planned session into the completed activity that fulfils it is the writer's responsibility (Garmin's scheduled-workout id and activity id differ); the API only provides the `status` field, the `status` filter, and PATCH/DELETE to support whatever reconciliation the writer chooses.

#### Scenario: POST a planned workout in the future is accepted

- **WHEN** the client posts `{"source":"garmin","sport":"bike","status":"planned","started_at":"<3 weeks from now>","ended_at":"<3 weeks from now + 2h>"}`
- **THEN** the system returns `201 Created` with `status: "planned"`
- **AND** the future-date guard does NOT reject it

#### Scenario: POST a completed workout in the future is still rejected

- **WHEN** the client posts a body with `status` omitted (or `"completed"`) and `started_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: A planned workout more than a year out is rejected

- **WHEN** the client posts `status: "planned"` with `started_at` more than 12 months in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: Invalid status value is rejected

- **WHEN** the client posts `status: "scheduled"` (not in the enum)
- **THEN** the system returns `400 Bad Request` with `{"error":"status_invalid"}`

#### Scenario: GET /workouts filters by status

- **WHEN** a window contains one `planned` and one `completed` workout
- **AND** the client calls `GET /workouts?from=…&to=…&status=planned`
- **THEN** only the planned workout is returned
- **AND** omitting `status` returns both (no implicit filter)
- **AND** the window parameters remain required

#### Scenario: PATCH can promote a planned workout to completed

- **WHEN** a `planned` workout exists
- **AND** the client patches `{"status":"completed"}`
- **THEN** the row's `status` becomes `completed`
- **AND** patching an invalid status value returns `400 status_invalid`

#### Scenario: Planned workouts do not distort energy or fueling aggregates

- **WHEN** a `planned` workout (no `kcal_burned`, future-dated) coexists with completed workouts
- **THEN** it contributes nothing to energy-availability burn sums (it has no `kcal_burned`)
- **AND** it does not appear inside any `GET /workouts/{id}/fueling` window for a real (completed) session
- **AND** read paths that must exclude plans filter on `status = 'completed'`

### Requirement: Planned workouts can originate from a training-plan slot via a slot-keyed upsert

The system SHALL support upserting a planned workout from a training-plan slot,
keyed on `plan_slot_id`. For a **single-sport** slot such a row SHALL carry
`status='planned'`, the slot's template's `sport` and `name`, a `template_id`,
and a `plan_slot_id`. For a **multisport** slot the row SHALL instead carry
`status='planned'`, `sport='multisport'`, the multisport template's `name`, a
`multisport_template_id`, and a `plan_slot_id` (no `template_id`). Because the key
is `plan_slot_id` and imported activities never carry one, this path is disjoint
from the existing `external_id` UPSERT path: the two never collide. Repeated
upserts on the same slot SHALL update the same row rather than create a new one.
The upsert's update SHALL apply only where the existing row's `status` is
`planned`, so a workout already marked `completed` is never reverted or
overwritten by re-materialization.

#### Scenario: The slot upsert does not overwrite a completed workout

- **WHEN** a planned workout for a slot has been marked `completed` and the slot
  is upserted again
- **THEN** the existing completed row is left unchanged (the update is guarded by
  `status='planned'`)

#### Scenario: A planned workout upserts by slot, not external_id

- **WHEN** the training-plan materializer upserts a planned workout for a given
  `plan_slot_id` twice
- **THEN** exactly one planned `workouts` row exists for that slot, updated in place

#### Scenario: A multisport slot upserts a multisport planned workout

- **WHEN** the materializer upserts a planned workout for a slot that references a
  multisport template
- **THEN** the slot-keyed row carries `sport='multisport'` and the
  `multisport_template_id`, with no `template_id`

#### Scenario: The slot-keyed and external_id paths do not collide

- **WHEN** a completed activity (with `external_id`, no `plan_slot_id`) and a
  planned workout (with `plan_slot_id`, no `external_id`) exist for the same date
- **THEN** both rows persist independently, each addressable by its own key

### Requirement: Workouts track Garmin scheduling identifiers

The system SHALL add two nullable columns to `workouts`: `garmin_workout_id`
(the id of the structured workout created in the Garmin library) and
`garmin_schedule_id` (the id of the calendar entry that schedules it). Both are
opaque Garmin identifiers — stored and echoed, never parsed. They are populated
when a planned workout is pushed to the watch and cleared when it is
unscheduled, enabling clean unschedule and re-push without double-creating in the
Garmin library.

#### Scenario: Columns exist after migration

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` has `garmin_workout_id` (TEXT NULL) and `garmin_schedule_id` (TEXT NULL)

#### Scenario: Ids are set on push and cleared on unschedule

- **WHEN** a planned workout is pushed to the watch and later unscheduled
- **THEN** both ids are populated by the push
- **AND** both ids are null after the unschedule

### Requirement: Garmin imports reconcile against open planned workouts

The system SHALL reconcile completed activities and planned workouts in **both
directions**, matching on exact sport and **local calendar day within a ±1-day
tolerance**, preferring an exact same-day candidate. A match SHALL act only when
there is **exactly one** candidate after same-day preference (if any candidate
falls on the exact day, only same-day candidates are considered); zero or
more-than-one SHALL never auto-link.

**Forward (at ingest).** When a completed activity is ingested via
`POST /workouts` or `POST /workouts/bulk` with `source='garmin'` and its
`external_id` is not already stored, the system SHALL match exactly one **open
planned workout** — `status='planned'`, `external_id IS NULL`, the same sport,
within ±1 local day of the activity's start (same day preferred). On exactly one
match the system SHALL **fulfill** that planned workout in place: set its
`external_id`, `source`, and actual metrics from the activity, flip `status` to
`completed`, and retain its `template_id` and `plan_slot_id`; no new row is
created. On no match the system SHALL insert a standalone completed row. On more
than one candidate the system SHALL insert a standalone completed row and mark it
as needing a link rather than guess. The match SHALL run only on first sight; a
subsequent re-sync of the same activity follows the existing `external_id` UPSERT
path.

**Reverse (at materialize).** When a plan slot is materialized and no workout row
yet exists for that `plan_slot_id`, the system SHALL match exactly one
**adoptable completed activity** — `status='completed'`, `plan_slot_id IS NULL`,
`external_id IS NOT NULL`, the slot's sport, within ±1 local day of the slot's
planned date (same day preferred). On exactly one match the system SHALL **adopt**
that activity: set its `plan_slot_id` and `template_id` from the slot, clear
`needs_link`, and keep its `status='completed'` and actual metrics; no planned
row is created. On no match the system SHALL create the planned row as usual. On
more than one candidate the system SHALL create the planned row and leave the
completed activities standalone. Once a slot has a workout row, re-materialize
follows the existing `plan_slot_id`-keyed, `status='planned'`-guarded path and
SHALL NOT re-adopt or duplicate.

#### Scenario: A completed import fulfills the matching planned workout

- **WHEN** a `garmin` activity is ingested for a sport and local day on which
  exactly one open planned workout exists
- **THEN** that planned workout is updated to `status='completed'` with the
  activity's `external_id`, `source`, and actual metrics
- **AND** its `template_id` and `plan_slot_id` are retained
- **AND** no second row is created

#### Scenario: No matching planned workout creates a standalone row

- **WHEN** a `garmin` activity is ingested and no open planned workout matches its
  sport within ±1 local day
- **THEN** a standalone completed workout is created (the prior behavior)

#### Scenario: Ambiguous match is flagged, not guessed

- **WHEN** a `garmin` activity matches more than one open planned workout of the
  same sport (after same-day preference)
- **THEN** a standalone completed workout is created and marked as needing a link
- **AND** no planned workout is auto-fulfilled

#### Scenario: Re-sync of a fulfilled activity is idempotent

- **WHEN** the daily sync re-sends an activity whose `external_id` is already
  stored (on a fulfilled planned row)
- **THEN** ingestion follows the existing `external_id` UPSERT path and updates
  that row in place
- **AND** reconciliation does not run again

#### Scenario: Matching uses local calendar day and exact sport

- **WHEN** an activity starts late in the local evening
- **THEN** it is matched against planned workouts on that local date (not the UTC
  date)
- **AND** only planned workouts of the same sport are considered

#### Scenario: A cross-day-by-one activity reconciles via the tolerance

- **WHEN** a `garmin` run is ingested whose local day is one day off the only
  open planned run of the same sport, and there is no same-day candidate
- **THEN** that adjacent-day planned workout is fulfilled (no standalone row)

#### Scenario: Same-day candidate is preferred over an adjacent-day one

- **WHEN** both a same-day and an adjacent-day open planned workout of the sport
  exist for an ingested activity
- **THEN** the same-day planned workout is fulfilled and the adjacent-day one is
  untouched (the match is not treated as ambiguous)

#### Scenario: Materialize adopts an already-imported activity (reverse)

- **WHEN** a plan slot is materialized and exactly one completed `garmin` activity
  of the slot's sport, with no `plan_slot_id`, exists within ±1 local day of the
  slot's date
- **THEN** that activity is adopted — its `plan_slot_id` and `template_id` are set
  from the slot, `needs_link` is cleared, and it stays `completed`
- **AND** no new planned row is created for the slot

#### Scenario: Reverse declines on more than one candidate

- **WHEN** a slot is materialized and more than one adoptable completed activity
  matches its sport within ±1 local day (after same-day preference)
- **THEN** the planned row is created normally and the completed activities are
  left standalone (resolved via explicit fulfill)

#### Scenario: Re-materialize does not re-adopt or duplicate

- **WHEN** a slot whose activity was already adopted (a completed row carrying its
  `plan_slot_id`) is re-materialized
- **THEN** the slot-keyed, `status='planned'`-guarded path skips it and no second
  row is created

### Requirement: Explicit fulfill and unfulfill endpoints

The system SHALL expose `POST /workouts/{plannedId}/fulfill` accepting a
`completed_id`, which merges an existing completed activity into an existing
planned workout (copying `external_id`, `source`, and actuals onto the planned
row, flipping it to `completed`, removing the redundant standalone row, and
clearing any needs-link flag); and `POST /workouts/{id}/unfulfill`, which
reverses a merge (clearing `external_id` and actuals and restoring
`status='planned'`). The planned row is the surviving identity in a merge so its
`plan_slot_id` remains stable.

#### Scenario: Manual fulfill merges two existing rows

- **WHEN** a client `POST`s `/workouts/{plannedId}/fulfill` with the id of a
  standalone completed activity of the same session
- **THEN** the planned workout becomes `completed` with the activity's
  `external_id` and actuals
- **AND** the standalone completed row is removed
- **AND** the planned workout's `plan_slot_id` is unchanged

#### Scenario: Unfulfill restores the planned workout

- **WHEN** a client `POST`s `/workouts/{id}/unfulfill` on a fulfilled workout
- **THEN** its `external_id` and actual metrics are cleared and `status` returns
  to `planned`
- **AND** the row retains its `template_id` and `plan_slot_id`

#### Scenario: Fulfill clears the needs-link flag

- **WHEN** a flagged (needs-link) completed activity is merged via `fulfill`
- **THEN** the needs-link flag is cleared on the surviving row

### Requirement: Workouts carry per-activity detail columns and child tables

The system SHALL persist richer per-activity detail alongside each workout: scalar performance fields, HR-zone time, and ambient-weather fields (humidity, wind — complementing the existing `temperature_c`) as columns on the `workouts` row, and per-lap splits and per-set strength data in dedicated child tables. This narrows the capability's prior "no performance analysis" exclusion to **streams/GPS only** — laps, HR-zone distribution, strength sets, and in-session weather are now in scope because they feed nutrition fueling math (carbohydrate-oxidation rate, glycogen cost, and especially sweat-rate, where humidity is a primary driver alongside temperature), not generic performance analytics. All detail is nullable/optional: "not measured" remains a meaningful state, never a data-quality bug.

#### Scenario: Detail columns are added to the workouts table

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` carries the additional nullable columns:
  - `elevation_gain_m` (NUMERIC(8, 1) NULL, CHECK `elevation_gain_m IS NULL OR elevation_gain_m >= 0`)
  - `elevation_loss_m` (NUMERIC(8, 1) NULL, CHECK `elevation_loss_m IS NULL OR elevation_loss_m >= 0`)
  - `normalized_power_w` (INTEGER NULL, CHECK `normalized_power_w IS NULL OR normalized_power_w > 0`)
  - `intensity_factor` (NUMERIC(4, 2) NULL, CHECK `intensity_factor IS NULL OR intensity_factor >= 0`)
  - `avg_cadence` (INTEGER NULL, CHECK `avg_cadence IS NULL OR avg_cadence > 0`)
  - `avg_stride_m` (NUMERIC(5, 2) NULL, CHECK `avg_stride_m IS NULL OR avg_stride_m > 0`)
  - `max_hr` (INTEGER NULL, CHECK `max_hr IS NULL OR max_hr > 0`)
  - `aerobic_te` (NUMERIC(3, 1) NULL, CHECK `aerobic_te IS NULL OR aerobic_te >= 0`)
  - `anaerobic_te` (NUMERIC(3, 1) NULL, CHECK `anaerobic_te IS NULL OR anaerobic_te >= 0`)
  - `secs_in_zone_1` … `secs_in_zone_5` (INTEGER NULL each, CHECK `IS NULL OR >= 0`)
  - `humidity_pct` (NUMERIC(5, 1) NULL, CHECK `humidity_pct IS NULL OR (humidity_pct BETWEEN 0 AND 100)`)
  - `wind_speed_mps` (NUMERIC(5, 1) NULL, CHECK `wind_speed_mps IS NULL OR wind_speed_mps >= 0`)
- **AND** every existing row carries NULL for all of them
- **AND** the migration succeeds without back-filling any of them

#### Scenario: workout_splits child table is created

- **WHEN** the migration is applied to a clean database
- **THEN** `workout_splits` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `workout_id` (UUID NOT NULL, REFERENCES `workouts(id)` ON DELETE CASCADE)
  - `split_index` (INTEGER NOT NULL, CHECK `split_index >= 0`)
  - `distance_m` (NUMERIC(10, 1) NULL)
  - `duration_s` (NUMERIC(10, 1) NULL)
  - `avg_hr` (INTEGER NULL)
  - `avg_power_w` (INTEGER NULL)
  - `avg_speed_mps` (NUMERIC(8, 3) NULL)
  - `elevation_gain_m` (NUMERIC(8, 1) NULL)
- **AND** an index `workout_splits_workout_id_idx` exists on `(workout_id)`
- **AND** a UNIQUE index exists on `(workout_id, split_index)`

#### Scenario: workout_sets child table is created

- **WHEN** the migration is applied to a clean database
- **THEN** `workout_sets` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `workout_id` (UUID NOT NULL, REFERENCES `workouts(id)` ON DELETE CASCADE)
  - `set_index` (INTEGER NOT NULL, CHECK `set_index >= 0`)
  - `exercise_name` (TEXT NULL)
  - `exercise_category` (TEXT NULL)
  - `reps` (INTEGER NULL, CHECK `reps IS NULL OR reps >= 0`)
  - `weight_kg` (NUMERIC(6, 2) NULL, CHECK `weight_kg IS NULL OR weight_kg >= 0`)
  - `duration_s` (NUMERIC(10, 1) NULL)
- **AND** an index `workout_sets_workout_id_idx` exists on `(workout_id)`
- **AND** a UNIQUE index exists on `(workout_id, set_index)`

#### Scenario: Detail floats are rounded at the response boundary

- **WHEN** a workout with detail is serialized to a response
- **THEN** every nutrient/measurement float in the scalar, zone, split, and set fields is rounded with `numfmt.Round1` at the boundary, consistent with the rest of the workouts shape
- **AND** the detail fields are NEVER merged into `summary`'s Totals struct (unit isolation preserved)

#### Scenario: List carries scalar and zone fields but never nested detail

- **WHEN** workouts in a window have detail columns set and child split/set rows
- **AND** the client calls `GET /workouts?from=…&to=…`
- **THEN** each listed workout includes the scalar performance and `secs_in_zone_*` fields when set (omitempty when NULL)
- **AND** no `splits` or `sets` arrays appear in the list response (nested detail is single-get only, to keep list payloads bounded)

#### Scenario: Detail attaches to the reconciled row, not a duplicate

- **WHEN** a planned workout exists for the day and a Garmin import carrying nested `splits`/`sets` reconciles into it (the existing reconciliation merges the import planned→completed in place, keeping `template_id`/`plan_slot_id`)
- **THEN** the scalar/zone columns and the child split/set rows attach to the surviving reconciled workout row (the planned row that was completed), NOT to a second inserted row
- **AND** a subsequent re-sync of the same activity replaces that row's children in place (no duplication across the reconcile + re-sync seam)
- **AND** the `external_id` lands on the reconciled row so future imports continue to match it

### Requirement: Bike intensity_factor is derived from FTP when missing

The system SHALL derive a workout's `intensity_factor` as `normalized_power_w / ftp_watts`, rounded to 2 decimal places, and store it on workout create and update — but ONLY when ALL of the following hold:

- the workout's `sport` is `bike`,
- `normalized_power_w` is present and `> 0`,
- the `athlete_config` singleton has `ftp_watts` present and `> 0`, and
- the caller did NOT explicitly supply a non-null `intensity_factor` in the request.

When the caller supplies an `intensity_factor`, that value SHALL be stored verbatim (rounded at the response boundary only) and the derivation SHALL NOT run — a watch- or client-provided IF always wins. When any gate condition fails (non-bike sport, missing/zero `normalized_power_w`, unset/zero `ftp_watts`, or the athlete-config dependency unavailable), the workout SHALL be written through unchanged with `intensity_factor` left as the caller provided it (NULL when absent), and no error SHALL be raised. The value is computed against the FTP in effect at write time; the system SHALL NOT retroactively recompute the stored value when `ftp_watts` later changes.

#### Scenario: Bike workout with NP and no supplied IF gets a derived value

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` is `0.80`

#### Scenario: Caller-supplied IF is never overridden

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and `intensity_factor` `0.95`
- **THEN** the stored `intensity_factor` is `0.95` (no derivation occurs)

#### Scenario: Non-bike workout is not derived

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `run` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL (FTP is a cycling metric; only bike sports derive)

#### Scenario: Missing FTP leaves IF NULL

- **WHEN** `athlete_config.ftp_watts` is unset (NULL)
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL and the create succeeds without error

#### Scenario: Missing normalized power leaves IF NULL

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with no `normalized_power_w` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL

#### Scenario: Update fills a previously-NULL IF

- **WHEN** a `bike` workout exists with `normalized_power_w` `200` and `intensity_factor` NULL
- **AND** `athlete_config.ftp_watts` is `250`
- **AND** the client updates (full-replace) that workout keeping `normalized_power_w` `200` and supplying no `intensity_factor`
- **THEN** the stored `intensity_factor` becomes `0.80`

### Requirement: A read computes plan-adherence analytics over a window

The system SHALL expose `GET /workouts/adherence` (authenticated) accepting
`from`, `to`, `tz`, and an optional `plan_id`, returning plan-adherence analytics
over the `[from, to]` local-date window. Each workout in the window SHALL be
classified once, from `status`, `plan_slot_id`, and `started_at` compared to the
current time in `tz`:

- **completed** — `status='completed'` with a `plan_slot_id` (a planned session
  that was done),
- **missed** — `status='planned'` with `started_at` before now (overdue),
- **upcoming** — `status='planned'` with `started_at` at or after now,
- **unplanned** — `status='completed'` with no `plan_slot_id` (off-plan work).

The response SHALL report the four counts, an **`adherence_rate`** equal to
`completed / (completed + missed)` rounded at the boundary and **null** when no
sessions are due (`completed + missed == 0`), a planned-vs-actual volume
(`planned_duration_min` over completed + missed sessions vs `completed_duration_min`
over completed sessions, and the same split for `tss` where present), and a
`by_sport` breakdown of completed/missed counts. When `plan_id` is supplied the
window SHALL be restricted to workouts whose `plan_slot_id` belongs to that plan
(joined through `plan_slots`/`plan_weeks`), which excludes unplanned rows. The
read SHALL NOT mutate any workout. Numeric fields SHALL be rounded at the response
boundary and a sum over zero present values SHALL serialize as null.

#### Scenario: Completed and missed sessions drive the adherence rate

- **WHEN** a window contains three planned sessions whose dates have passed —
  two fulfilled (`status='completed'`, `plan_slot_id` set) and one still
  `planned` — and `GET /workouts/adherence` is called
- **THEN** `completed` is 2, `missed` is 1, and `adherence_rate` is `0.7` (2 / 3)

#### Scenario: Upcoming sessions are excluded from the rate

- **WHEN** the window also contains a `planned` session dated in the future
- **THEN** it is counted under `upcoming` and does NOT change `completed`,
  `missed`, or `adherence_rate`

#### Scenario: Off-plan completed work is unplanned, not counted against adherence

- **WHEN** a completed workout with no `plan_slot_id` falls in the window
- **THEN** it is counted under `unplanned` and excluded from `adherence_rate`

#### Scenario: Adherence rate is null when nothing is due

- **WHEN** the window contains only upcoming planned sessions (none overdue, none
  completed)
- **THEN** `adherence_rate` is null and `completed`/`missed` are 0

#### Scenario: Planned-vs-actual volume and by-sport are reported

- **WHEN** the window has completed and missed sessions across sports
- **THEN** the response reports `planned_duration_min` (completed + missed) and
  `completed_duration_min` (completed only), and a `by_sport` map of
  completed/missed counts per sport

#### Scenario: plan_id scopes the window to one plan

- **WHEN** `GET /workouts/adherence` is called with a `plan_id`
- **THEN** only workouts whose `plan_slot_id` belongs to that plan are considered
- **AND** off-plan (no-slot) completed workouts are excluded

#### Scenario: The adherence read is exposed as an MCP tool

- **WHEN** the agent calls the adherence MCP tool
- **THEN** the MCP server issues exactly one `GET /workouts/adherence` request and
  forwards the response verbatim

### Requirement: Workouts carry an optional training_focus classification

The system SHALL support an optional `training_focus` field on a workout that classifies
the session's intensity band against the 7-zone Trainingsbereiche model. The allowed
values are exactly `recovery`, `basic_endurance_1`, `basic_endurance_2`, `development`,
`competition_specific`, `peak`, and `strength_endurance` (corresponding to REKOM, GA1,
GA2, EB, WSA, SB, KA respectively). The field is nullable — an unclassified session is a
valid state, not a data-quality defect — and is stored in a `training_focus TEXT` column
on the `workouts` table guarded by a CHECK constraint that admits the 7 values or NULL.
The field is accepted and validated on `POST /workouts`, on each item of
`POST /workouts/bulk`, and on `PATCH /workouts/{id}`; it is returned on `GET /workouts`
and `GET /workouts/{id}` following the `omitempty` pattern. Validation failures map to a
`training_focus_invalid` error code. The field is NOT derived from HR zones, power, or
TSS — it is an explicit annotation, independent of the `secs_in_zone_*` actuals.

#### Scenario: training_focus column is nullable with no back-fill

- **WHEN** the migration adding `training_focus` is applied to a database with existing `workouts` rows
- **THEN** every existing row carries `training_focus = NULL`
- **AND** the migration succeeds without back-filling the column
- **AND** subsequent INSERT/UPSERT/PATCH paths default the field to NULL when omitted
- **AND** the CHECK constraint accepts NULL and each of the 7 enum values, and rejects any other string

#### Scenario: POST with a valid training_focus stores the value

- **WHEN** the client posts `{"source":"manual","sport":"bike","started_at":"2026-07-15T08:00:00Z","ended_at":"2026-07-15T09:30:00Z","training_focus":"basic_endurance_1"}`
- **THEN** the system creates a row with `training_focus = 'basic_endurance_1'`
- **AND** returns `201 Created` with the response body echoing the field

#### Scenario: POST omitting training_focus stores NULL

- **WHEN** the client posts a workout body that omits `training_focus`
- **THEN** the row is created with `training_focus = NULL`
- **AND** the response body's JSON omits the field (omitempty pattern matching `tss`, `rpe`)

#### Scenario: POST with an unknown training_focus value is rejected

- **WHEN** the client posts a workout body with `training_focus` set to `"ga1"`, `"zone2"`, `"sweet_spot"`, or any string outside the 7 allowed values
- **THEN** the system returns `400 Bad Request` with `{"error":"training_focus_invalid"}`
- **AND** no row is inserted

#### Scenario: All seven enum values are accepted

- **WHEN** the client posts workouts with `training_focus` set in turn to each of `recovery`, `basic_endurance_1`, `basic_endurance_2`, `development`, `competition_specific`, `peak`, `strength_endurance`
- **THEN** each POST succeeds and stores the supplied value verbatim

#### Scenario: training_focus is independent of sport

- **WHEN** the client posts `{"source":"manual","sport":"strength",…,"training_focus":"strength_endurance"}` and separately `{"source":"manual","sport":"run",…,"training_focus":"competition_specific"}`
- **THEN** both are accepted — `training_focus` is validated only against the enum, with no sport-coupling

#### Scenario: GET returns training_focus when set

- **WHEN** the client requests a workout whose row has a non-NULL `training_focus`
- **THEN** the response body includes `training_focus` with the stored value
- **AND** a workout with `training_focus = NULL` omits the field from the response

#### Scenario: PATCH sets training_focus on an existing workout

- **WHEN** the client `PATCH`es `{"training_focus":"competition_specific"}` on an existing workout
- **THEN** the row's `training_focus` becomes `'competition_specific'`
- **AND** other fields are unchanged

#### Scenario: PATCH absent training_focus leaves it unchanged

- **WHEN** the client `PATCH`es a body that does not mention `training_focus`
- **THEN** the existing `training_focus` value is preserved

#### Scenario: PATCH explicit null clears training_focus to NULL

- **WHEN** the client `PATCH`es `{"training_focus":null}` on a workout that currently has a value
- **THEN** the row's `training_focus` is set to NULL
- **AND** the field is omitted from the subsequent GET response

#### Scenario: PATCH with an unknown training_focus value is rejected

- **WHEN** the client `PATCH`es `{"training_focus":"tempo"}` on an existing workout
- **THEN** the system returns `400 Bad Request` with `{"error":"training_focus_invalid"}`
- **AND** the stored `training_focus` is left unchanged

#### Scenario: Bulk upsert validates training_focus per item

- **WHEN** a `POST /workouts/bulk` batch contains one item with a valid `training_focus`, one omitting it, and one with an invalid value
- **THEN** the valid item is stored with its value, the omitting item is stored with NULL, and the invalid item is reported as a per-item `training_focus_invalid` failure
- **AND** the overall response is `200 OK` (partial failure allowed)

### Requirement: The adherence read lists the missed sessions

The `GET /workouts/adherence` response SHALL include a **`missed_sessions`** array
naming the sessions classified as **missed** (`status='planned'` with `started_at`
before the current time in `tz`). Each entry SHALL be compact — `id`, `date` (the
session's local date), `sport`, `planned_duration_min`, and `planned_tss` (null
when the planned session carries no TSS) — and the array SHALL be ordered by date
ascending.

The array SHALL be capped at a fixed maximum. When the number of missed sessions
exceeds the cap the response SHALL drop the tail and set
**`missed_sessions_truncated`** to `true`; otherwise `missed_sessions_truncated`
SHALL be `false`. The list SHALL contain only missed sessions — not upcoming and
not unplanned work. Under `plan_id` scoping the list SHALL be restricted to that
plan's sessions, consistent with the counts.

#### Scenario: Missed sessions are named compactly

- **WHEN** a window contains two overdue `planned` sessions that were never
  fulfilled — a 60-minute run and a 90-minute ride — and `GET /workouts/adherence`
  is called
- **THEN** `missed_sessions` contains two entries, oldest first, each with `id`,
  `date`, `sport`, `planned_duration_min`, and `planned_tss`, and
  `missed_sessions_truncated` is `false`

#### Scenario: Only missed sessions appear in the list

- **WHEN** the window also contains a fulfilled (completed-from-plan) session, an
  upcoming `planned` session, and an off-plan completed workout
- **THEN** none of those three appear in `missed_sessions`

#### Scenario: An oversized list is truncated with an explicit flag

- **WHEN** the number of missed sessions in the window exceeds the cap
- **THEN** `missed_sessions` contains exactly the cap's worth of entries (the
  oldest), and `missed_sessions_truncated` is `true`

### Requirement: The adherence read reports a per-week trend

The `GET /workouts/adherence` response SHALL include a **`weekly`** array giving
per-week adherence over the window. Each bucket SHALL report `week_start` (the
local date of the week's first day), `completed`, `missed`, an `adherence_rate`
equal to `completed / (completed + missed)` rounded at the boundary and **null**
when nothing was due that week, and `planned_duration_min` (over that week's
completed + missed sessions) and `completed_duration_min` (over that week's
completed sessions). A bucket SHALL be emitted only for a week that contains at
least one candidate session; empty weeks SHALL NOT be zero-filled.

Bucketing SHALL be plan-week-aware. When `plan_id` is supplied, sessions SHALL be
grouped by their plan week (`plan_weeks.ordinal`), each bucket additionally
reporting `ordinal` and the week's phase name (null when the week has no phase),
with `week_start` derived from the plan's `start_date`. When `plan_id` is absent,
sessions SHALL be grouped by calendar week starting Monday, and `ordinal` and
phase SHALL be null. The trend SHALL be consistent with the top-level counts —
each classified session contributes to exactly one bucket and to the window total.

#### Scenario: Calendar-week trend without a plan

- **WHEN** `GET /workouts/adherence` is called over a multi-week window with no
  `plan_id`, and sessions fall across two Monday-started calendar weeks
- **THEN** `weekly` has one bucket per week that contains sessions, each with
  `week_start`, counts, `adherence_rate`, `planned_duration_min`, and
  `completed_duration_min`, and `ordinal`/`phase` are null

#### Scenario: Plan-week trend aligns to the plan's weeks

- **WHEN** `GET /workouts/adherence` is called with a `plan_id` spanning two plan
  weeks
- **THEN** each `weekly` bucket reports the plan week's `ordinal`, its phase name
  (or null), and a `week_start` derived from the plan's `start_date`, and off-plan
  work is excluded

#### Scenario: A week with only future sessions has a null rate

- **WHEN** a week in the window contains only `planned` sessions dated in the
  future
- **THEN** that bucket's `adherence_rate` is null and its `missed` is 0

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

### Requirement: Workouts carry stream-derived execution-metric columns

The system SHALL add three nullable columns to `workouts`, written exclusively by the
`activity-streams` ingest/recompute path (never accepted on `POST /workouts`,
`POST /workouts/bulk`, or `PATCH /workouts/{id}` — they are not part of the mutable
field set):

- `variability_index` (NUMERIC(4, 2) NULL, CHECK `variability_index IS NULL OR variability_index > 0`)
- `efficiency_factor` (NUMERIC(6, 3) NULL, CHECK `efficiency_factor IS NULL OR efficiency_factor > 0`)
- `decoupling_pct` (NUMERIC(5, 1) NULL, CHECK `decoupling_pct IS NULL OR (decoupling_pct BETWEEN -100 AND 100)`)

Every existing row SHALL carry NULL for all three (no back-fill — historical workouts
have no stored streams until re-synced). Read paths (`GET /workouts`,
`GET /workouts/{id}`) SHALL echo the three fields under the standard omitempty rule:
present when set, keys absent when NULL. The values are unit-isolated performance
signals and SHALL feed no nutrition, hydration, or energy total.

#### Scenario: Columns are added nullable with no back-fill

- **WHEN** the migration adding the three execution-metric columns is applied to a
  database with existing `workouts` rows
- **THEN** every existing row carries NULL for `variability_index`,
  `efficiency_factor`, and `decoupling_pct`
- **AND** the migration succeeds without back-filling any of them

#### Scenario: GET echoes execution metrics when set

- **WHEN** a workout's streams have been ingested and its metrics derived
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body includes `variability_index`, `efficiency_factor`, and
  `decoupling_pct` with their derived values

#### Scenario: GET omits execution metrics when NULL

- **WHEN** a workout has no derived execution metrics
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body omits all three keys (omitempty)

#### Scenario: Execution metrics are not client-writable

- **WHEN** the client sends `variability_index`, `efficiency_factor`, or
  `decoupling_pct` in a `POST /workouts` or `PATCH /workouts/{id}` body
- **THEN** the fields are not part of the accepted request shape and are never written
  from client input (the only writers are the stream ingest and recompute paths)

