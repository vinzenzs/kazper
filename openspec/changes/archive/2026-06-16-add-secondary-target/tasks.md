## 1. Step model + pair validation

- [x] 1.1 Add `SecondaryTarget *Target` (omitempty) to `workouttemplates.Step`.
- [x] 1.2 Validate at the template service layer: `secondary_target` accepted only on bike templates; reject `kind:"none"`; reject when primary and secondary share a metric family (power/hr/pace/rpe); validate each target with the existing Target validator.
- [x] 1.3 Reject `secondary_target` inside repeat-group children the same way (validation walks nested steps).

## 2. Garmin bridge emission

- [x] 2.1 Add `_secondary_target(target)` to `workout_builder.py` emitting `secondaryTargetType` + `secondaryZoneNumber`/`secondaryTargetValueOne`/`secondaryTargetValueTwo`, reusing per-kind value logic.
- [x] 2.2 Merge it into `_build_step` when `secondary_target` is present. Verify the exact garminconnect `secondaryTarget*` field names against the library.
- [x] 2.3 Unit-test: a bike step with primary power_zone + secondary hr_zone produces both target sets.

## 3. MCP tool surface

- [x] 3.1 Add `secondary_target` to the `create_workout_template`/`patch_workout_template` step arg schema (`registry_workouttemplates.go`) with jsonschema docs noting bike-only.
- [x] 3.2 Update the tool description to mention primary + secondary bike targets.

## 4. Resolver coupling (only if `resolve-zone-targets` has shipped)

- [x] 4.1 If the zone resolver exists, extend it to also resolve a zone-kind `secondary_target` (bike `power_zone`/`hr_zone` → absolute), with the same bike-gate and passthrough rules as the primary. Add a resolver scenario for a secondary power_zone.

## 5. Tests + docs

- [x] 5.1 Integration test: create a bike template with primary+secondary, echoed verbatim; assert non-bike secondary and same-family secondary are rejected.
- [x] 5.2 Integration test: `GET /workouts/{id}/program` carries the secondary target through the effective program.
- [x] 5.3 Run `task swag` (Step shape change), then `task test` and `task vet`.
