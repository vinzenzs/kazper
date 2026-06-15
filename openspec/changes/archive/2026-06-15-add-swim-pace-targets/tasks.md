## 1. Target model + validation

- [x] 1.1 Add `LowSecPer100m`/`HighSecPer100m` (`*int`, omitempty) to `workouttemplates.Target` and `swim_pace` to the allowed kinds.
- [x] 1.2 Validate `swim_pace`: positive bounds, `low <= high`; reject `swim_pace` on non-swim templates and `pace` (`/km`) on swim templates (sport is known at the template service layer).
- [x] 1.3 Ensure the same Target validator is reused by `trainingplan` slot `target_overrides` (already shared) so `swim_pace` overrides validate identically.

## 2. Garmin bridge conversion

- [x] 2.1 Add a `swim_pace` branch to `workout_builder._target` converting `100 / sec_per_100m` → m/s, reusing the Garmin pace target type.
- [x] 2.2 Unit-test the conversion (e.g. 100 s/100m → 1.0 m/s; inverted/zero guarded).

## 3. MCP tool surface

- [x] 3.1 Add `low_sec_per_100m`/`high_sec_per_100m` to the `wtTarget` arg schema (`registry_workouttemplates.go`) and the slot override arg (`registry_trainingplan.go`) with jsonschema docs and a swim example.
- [x] 3.2 Update the `create_workout_template` / `add_plan_slot` tool descriptions to mention swim pace.

## 4. Tests + docs

- [x] 4.1 Integration test: create a swim template with a `swim_pace` step; assert echoed verbatim; assert non-swim `swim_pace` and swim `pace` are rejected.
- [x] 4.2 Integration test: a swim slot `target_overrides` with `swim_pace` resolves into the effective program (`GET /workouts/{id}/program`).
- [x] 4.3 Run `task swag` (Target shape change), then `task test` and `task vet`.
