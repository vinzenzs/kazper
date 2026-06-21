## ADDED Requirements

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
