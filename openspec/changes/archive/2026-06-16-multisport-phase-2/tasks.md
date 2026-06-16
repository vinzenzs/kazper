## 1. Migration

- [x] 1.1 Add migration `046_add_multisport_plan_integration` (verify head is `045` first): on `plan_slots` drop `template_id` NOT NULL, add `multisport_template_id UUID NULL REFERENCES multisport_templates(id) ON DELETE RESTRICT`, and add a CHECK that exactly one of `template_id`/`multisport_template_id` is non-null.
- [x] 1.2 Same migration: on `workouts` add `multisport_template_id UUID NULL REFERENCES multisport_templates(id) ON DELETE SET NULL` and widen the `sport` CHECK to include `'multisport'`. Write the `.down.sql` (drop columns/constraints, restore `template_id NOT NULL`).

## 2. Workouts capability

- [x] 2.1 Add `SportMultisport = "multisport"` to `internal/workouts` sport vocabulary + `ValidSport`/`ParseSport`; do NOT add `transition` (segment-only).
- [x] 2.2 Add `MultisportTemplateID *uuid.UUID` to the `Workout` row type + repo scan/columns, and to the `PlannedSlotInput`; teach `UpsertPlannedFromSlot` to write `sport='multisport'`, `multisport_template_id`, null `template_id` when the input is multisport.
- [x] 2.3 Update the workouts repo INSERT/SELECT column lists and any handlers/swagger structs that enumerate sport or the template link.

## 3. Plan slot template reference (XOR)

- [x] 3.1 Add `MultisportTemplateID *uuid.UUID` to `PlanSlot`, `SlotInput`, and `SlotPatch` in `internal/trainingplan`; relax the single-sport `template_id` to optional.
- [x] 3.2 Validate XOR in `CreateSlot`/`PatchSlot`: exactly one of `template_id`/`multisport_template_id` set (new sentinel, e.g. `ErrTemplateAmbiguous`); reject `target_overrides`/`duration_overrides` on a multisport slot.
- [x] 3.3 Update the slot repo (load/insert/patch + the materialize-slot join) to carry `multisport_template_id` and the multisport template's name/sport context.

## 4. EffectiveProgram + per-segment resolution

- [x] 4.1 Cross-inject the `multisport` repo into `trainingplan.Service` (`SetMultisportRepo`, nil-safe), wired in `internal/httpserver/server.go`.
- [x] 4.2 Add `Segments []ProgramSegment` (`omitempty`) to `trainingplan.Program` (`{sport, steps, duration?}` per segment).
- [x] 4.3 In `EffectiveProgram`, branch on `w.Sport == "multisport"`: load the multisport template, run the existing `resolveTargets` per segment keyed by the segment's sport (bike resolves `power_zone`/secondary, run/swim pass through), return `Program{Sport:"multisport", Segments:[…]}`; keep the single-sport path byte-for-byte unchanged.

## 5. Materialize

- [x] 5.1 In `Materialize`, branch per slot: a multisport slot upserts a `sport='multisport'` planned workout referencing the multisport template; derive session length from the summed segment step durations (fallback to summed segment durations, then 1h).
- [x] 5.2 Confirm slot-keyed idempotency + `status='planned'` guard hold for multisport rows.

## 6. Garmin push path

- [x] 6.1 In `internal/garmincontrol` `pushOne`/`bridgeCreateWorkout`, send the multisport form (segments) to the bridge when `prog.Segments` is non-empty; reuse Phase 1's multi-segment compile (no bridge change).
- [x] 6.2 Ensure `get_workout_program` returns `Segments` for a multisport workout (handler + response shape).

## 7. MCP / REST surface

- [x] 7.1 Add `multisport_template_id` to the slot create/patch tool args in `internal/agenttools/registry_trainingplan.go` (XOR with `template_id`, documented); no new tools.
- [x] 7.2 Regenerate the MCP announced-schema golden baseline (slot tool arg schemas grew; mirrors the swim_pace/cadence precedent) and confirm the expected-tools list is unchanged.

## 8. Tests

- [x] 8.1 Unit/integration: slot XOR validation (neither/both rejected; multisport slot accepted; overrides on a multisport slot rejected).
- [x] 8.2 Integration: materialize a multisport slot → `sport='multisport'` planned row with `multisport_template_id`; idempotent; never reverts a completed row.
- [x] 8.3 Integration: `GET /workouts/{id}/program` for a multisport workout returns ordered segments with per-segment-sport resolution (bike `power_zone`→`power_w`, run passthrough).
- [x] 8.4 Garmin push: a materialized multisport workout sends the multisport form (multiple `workoutSegments`) to the bridge stub.

## 9. Docs & verification

- [x] 9.1 Run `task swag` to regenerate `docs/` for the slot + workout response shape changes.
- [x] 9.2 Run `task test` and `task vet`; confirm the MCP integration expected-tools list still passes (no new tools).
