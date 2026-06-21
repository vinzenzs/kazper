## 1. Migration

- [x] 1.1 `task migrate:new NAME=add_workout_training_focus`; verify `047` is still the head before claiming `048`.
- [x] 1.2 In `048_*.up.sql`: `ALTER TABLE workouts ADD COLUMN training_focus TEXT CHECK (training_focus IN ('recovery','basic_endurance_1','basic_endurance_2','development','competition_specific','peak','strength_endurance'));` (NULL passes the CHECK; no DEFAULT, no back-fill).
- [x] 1.3 In `048_*.down.sql`: `ALTER TABLE workouts DROP COLUMN training_focus;`

## 2. Types

- [x] 2.1 In `internal/workouts/types.go`: add `TrainingFocus string` type with a const block for the 7 values (each const's doc comment notes its German code: REKOM/GA1/GA2/EB/WSA/SB/KA).
- [x] 2.2 Add `ValidTrainingFocus(s string) bool` (switch) and `ParseTrainingFocus(s string) (TrainingFocus, error)`, mirroring `ValidSport`/`ParseSport`.
- [x] 2.3 Add `TrainingFocus *TrainingFocus json:"training_focus,omitempty"` to the `Workout` struct (nullable pointer, omitempty — follow the `tss`/`rpe` field placement).

## 3. Service

- [x] 3.1 Add `ErrTrainingFocusInvalid = errors.New("training_focus_invalid")` to the sentinel block.
- [x] 3.2 Thread `TrainingFocus *string` onto `CreateInput`; in `buildWorkout`, validate via `ValidTrainingFocus` when non-nil and set the field, else leave NULL.
- [x] 3.3 Add tri-state plumbing to `PatchParams`/`PatchInput` (`TrainingFocus *string` + `ClearTrainingFocus bool`), validating on set; copy the existing nullable-field pattern used by `tss`/ingestion metrics.

## 4. Repo

- [x] 4.1 Add `training_focus` to `selectCols` and to row scanning.
- [x] 4.2 Add the column to the Upsert INSERT column/value list and the ON CONFLICT UPDATE set (full-replace semantics).
- [x] 4.3 Add `training_focus` to the PATCH dynamic SET builder, honoring set vs. clear-to-NULL.

## 5. Handlers

- [x] 5.1 Accept `training_focus` in the POST/bulk request struct; on `ErrTrainingFocusInvalid` return `400 {"error":"training_focus_invalid"}`.
- [x] 5.2 Accept tri-state `training_focus` in the PATCH request struct (absent / value / explicit `null`), converting `null` to `ClearTrainingFocus = true`; same error mapping.
- [x] 5.3 Ensure swag annotations on the request/response structs reflect the new field.

## 6. Tests

- [x] 6.1 POST stores a valid value; POST omitting it stores NULL (omitempty in response); POST with an unknown value → `400 training_focus_invalid`, no row.
- [x] 6.2 All 7 enum values accepted; training_focus accepted independent of sport.
- [x] 6.3 GET (list + by-id) echoes the value when set and omits when NULL.
- [x] 6.4 PATCH set / absent-unchanged / explicit-null-clears / invalid-rejected-without-touching-other-fields.
- [x] 6.5 Bulk: mixed batch (valid / omitted / invalid) → per-item results, overall `200`.

## 7. Docs & verification

- [x] 7.1 `task swag` to regenerate `docs/` from the updated structs.
- [x] 7.2 `task test` (workouts package green) and `task vet`.
