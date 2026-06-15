## Why

Garmin bike steps carry a **Primary + Secondary** target (e.g. hold Power Zone 3 *and* a cadence/HR band simultaneously), but our `Step.Target` is a single slot — so a bike interval can prescribe power *or* HR, never both. That loses the most common cycling prescription (power + cadence, or power + HR floor). Secondary targets are bike-only in Garmin, so this is a bike-scoped structural addition.

## What Changes

- Add an optional `secondary_target` slot (the same `Target` shape) to a workout-template step, **accepted only on bike templates**.
- Validate the pair: `secondary_target` rejected on non-bike steps; rejected if `kind:"none"`; primary and secondary MUST be in different metric families (no power+power).
- Teach the garmin-bridge `workout_builder` to emit Garmin's `secondaryTargetType` / `secondaryZoneNumber` / `secondaryTargetValueOne|Two` from the secondary slot.
- The effective program carries the secondary target through verbatim (it is not slot-overridable in this change — secondary is template-intrinsic).
- **Compatibility note**, not a code change here: if `resolve-zone-targets` has shipped, its zone→absolute resolver MUST also run over the secondary slot (a secondary `power_zone` on bike resolves the same way). Captured as a task gated on that change's state.
- Out of scope: secondary targets on run/swim (Garmin doesn't offer them); slot `target_overrides` for the secondary slot; the `cadence` target kind itself (separate `add-cadence-target` — until it lands, the usable secondary pairing is power+HR).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workout-templates`: a step gains an optional bike-only `secondary_target`, validated against the primary.
- `garmin-bridge`: the workout builder emits Garmin secondary-target fields from the secondary slot.

## Impact

- **Code**: `internal/workouttemplates/` (Step field + pair validation), `apps/garmin-bridge/garmin_bridge/workout_builder.py` (secondary-target emission), `internal/agenttools/registry_workouttemplates.go` (tool arg schema/docs). `internal/trainingplan/` carries it through unchanged.
- **Docs**: `task swag` for the Step shape change.
- **No migration** — steps are JSON-stored; the field is additive.
- **Ordering**: extends the same step contract that `resolve-zone-targets` resolves over and that `add-multisport-structured-workouts` would touch — land `resolve-zone-targets` first so the resolver can be extended to the secondary slot in one place.
