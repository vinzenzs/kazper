# Tasks

## 1. Backend

- [x] 1.1 Verify migration head (064 at proposal), then `task migrate:new NAME=add_workout_environment`: nullable CHECK column, no back-fill
      _Head on disk verified as `064_add_garmin_detected_thresholds` → this took slot **065**. The scaffold emitted a 6-digit `000065_` prefix; renamed to `065_` to match the recent head's convention (both parse identically, but the head's style wins)._
- [x] 1.2 `internal/workouts`: type (omitempty), POST/bulk acceptance, PATCH tri-state (empty-string clear), `environment_invalid`; confirm reconciliation Merge/RestorePlanned leave it untouched
      _**Spec/design conflict resolved by accepting both clearing forms.** The delta spec says the empty string clears ("the established sentinel convention"); the design says "`training_focus` conventions throughout". These contradict: `workouts`' PATCH decodes every nullable field via `json.RawMessage` and clears with **JSON null** — `training_focus` included. The empty-string sentinel belongs to meals/hydration/workoutfuel `workout_id`, where a plain `*string` can't tell null from missing. So `environment` accepts **both** `""` (the spec's literal contract, tested) and `null` (this endpoint's idiom for every sibling field). Neither is ambiguous — `""` is not a valid enum value. On CREATE, `""` is `environment_invalid`: the empty-string clear is a PATCH affordance, not a way to spell "not stated" on a write that can just omit the key._
      _Reconciliation confirmed untouched by inspection: neither `Merge` nor `RestorePlanned` names the column. Pinned with a test so a future column-list edit can't silently wipe it. (A merged row carries an `external_id`, so the next re-sync re-derives environment from Garmin anyway — the actual's truth arrives without special-casing merge.)_
- [x] 1.3 Integration tests: set/clear/omit tri-state, invalid 400, bulk, reconciliation preservation
- [x] 1.4 `task swag`

## 2. Bridge

- [x] 2.1 Activity-type → environment mapping (unit-tested table; unknown → omitted); include in bulk items
      _Table is **deliberately partial**: only type keys that *state* the answer map. A bare `cycling`/`running`/`swimming` could be either (rollers vs road, pool vs lake), so those omit the key and the backend stores null rather than guessing — a wrong label silently poisons acclimatization and the heat analytics, while null is honest and PATCHable._
- [x] 2.2 Bridge tests: indoor/virtual/treadmill/pool/openwater fixtures + unknown type; pytest green
      _163 pass (was 157): +10-case derivation table, omitted-for-ambiguous-types, unknown-type-still-syncs, missing-activityType-doesn't-crash._

## 3. Verification

- [x] 3.1 `task vet` + workouts suite green
- [ ] 3.1b Live: re-sync window fills recent workouts, spot-check a known trainer ride and a road ride
      _(operator step — needs a real Garmin sync; not runnable in-session.)_
      _**No deploy-ordering constraint** (checked, unlike the `add-cadence-quadrant-analysis` precedent): POST binds with Gin's `ShouldBindJSON` and bulk with a plain `json.Unmarshal`, neither of which rejects unknown fields — a new bridge against an old backend simply has `environment` ignored, and the field stays null until the backend catches up. The cadence change needed backend-first because its new stream type hit a DB CHECK; a plain ignored JSON key does not._
