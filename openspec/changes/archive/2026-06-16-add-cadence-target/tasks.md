## 1. Target model + validation

- [x] 1.1 Add `cadence` to the allowed `Target` kinds (reusing `Low`/`High`).
- [x] 1.2 Validate `cadence`: positive bounds, `low <= high`; accept only on bike/run templates (reject on swim/strength/yoga/mobility/other). Reuse for slot `target_overrides` via the shared validator.

## 2. Garmin bridge emission

- [x] 2.1 Add `"cadence": (3, "cadence")` to `_TARGET_TYPE` and emit `targetValueOne=low`/`targetValueTwo=high` (same branch as `hr_bpm`/`power_w`).
- [x] 2.2 Verify whether Garmin needs a sport-specific cadence key (`cadence` vs `run.cadence`); if so, select by segment/workout sport. Unit-test the emission.

## 3. MCP tool surface

- [x] 3.1 Update the `wtTarget` arg schema doc (`registry_workouttemplates.go`) and slot override arg (`registry_trainingplan.go`) to list `cadence` with a bike/run example.

## 4. Compose with secondary-target (gated)

- [x] 4.1 If `add-secondary-target` has shipped: register `cadence` as a distinct metric family in its pair validator so bike primary power + secondary cadence is allowed.

## 5. Tests + docs

- [x] 5.1 Integration test: run/bike template with a `cadence` target echoed verbatim; assert swim/strength `cadence` rejected and inverted range rejected.
- [x] 5.2 Run `task swag`, `task test`, `task vet`.
