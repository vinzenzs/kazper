## Context

`workouttemplates.Step` has a single `Target *Target`. The garmin-bridge `_build_step` calls `_target(node["target"])` which emits `targetType` + `zoneNumber`/`targetValueOne`/`targetValueTwo`. Garmin's `ExecutableStepDTO` additionally accepts `secondaryTargetType` + `secondaryZoneNumber`/`secondaryTargetValueOne`/`secondaryTargetValueTwo` — a second simultaneous target, offered only for bike steps in the Connect UI. We model none of this.

## Goals / Non-Goals

**Goals:**
- A bike step can prescribe two simultaneous targets (e.g. power + HR; power + cadence once cadence exists).
- The pair pushes to the watch as Garmin primary + secondary gates.

**Non-Goals:**
- Run/swim secondary targets (Garmin doesn't offer them).
- Slot `target_overrides` for the secondary slot (secondary is template-intrinsic; primary is what progresses weekly).
- The `cadence` target kind (separate `add-cadence-target`).
- Multisport (separate).

## Decisions

### D1: One `secondary_target` slot, reusing `Target`
Add `SecondaryTarget *Target` to `Step` (omitempty), not a generic `[]Target`. Mirrors Garmin's exactly-one-secondary model and the existing single-`Target` shape; avoids open-ended target lists the bridge couldn't map.

### D2: Bike-only, at the template service layer
`secondary_target` SHALL be rejected unless the template sport is `bike` (sport is known at validation). Keeps the model honest with Garmin's capability and avoids emitting fields the watch ignores for run/swim.

### D3: Pair validation — different metric families
Define metric families: power (`power_zone`/`power_w`), hr (`hr_zone`/`hr_bpm`), pace, cadence (future), rpe. Primary and secondary MUST be in **different** families; `secondary_target.kind` MUST NOT be `none`. Rejects nonsense (power+power) while allowing power+HR today and power+cadence later. Each target individually validated by the existing Target validator.

### D4: Bridge emits `secondary*` fields
Add a `_secondary_target(target)` in `workout_builder` that produces `secondaryTargetType` + `secondaryZoneNumber`/`secondaryTargetValueOne`/`secondaryTargetValueTwo`, reusing the same per-kind value logic as `_target`. `_build_step` merges it when `secondary_target` is present.

### D5: Effective program carries it through; resolver extension is gated
`EffectiveProgram`/`applyOverrides` copy the `Step` verbatim, so `secondary_target` flows to display and the bridge with no training-plan change. **However**, if `resolve-zone-targets` has shipped, its resolver only rewrites the primary `Target`; it MUST be extended to also resolve a zone-kind `secondary_target` (e.g. secondary `power_zone` on bike → `power_w`). This is the single coupling point between the two changes — handled as a task whose presence depends on whether the resolver exists at apply time.

## Risks / Trade-offs

- **Coupling with `resolve-zone-targets`** → Mitigation: D5 makes the resolver-extension explicit and bike-scoped (same gate as primary); if the resolver isn't shipped yet, nothing to do.
- **Most valuable pairing (power+cadence) needs `add-cadence-target`** → Mitigation: power+HR is usable immediately; cadence slots in later with no shape change.
- **Garmin secondary-field names** → Verify exact `secondaryTarget*` keys against garminconnect when implementing (task 2.x); the builder is the only place they appear.

## Migration Plan

Additive JSON field; no DB migration. Deploy code + `task swag`. Rollback = revert; no template uses `secondary_target` yet.

## Open Questions

- Should secondary targets ever be slot-overridable (e.g. progress a secondary cadence band)? Deferred — revisit if a real progression need appears.
