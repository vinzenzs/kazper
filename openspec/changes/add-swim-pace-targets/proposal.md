## Why

Swim pace targets are unexpressible today. The `Target` struct only carries `low_sec_per_km`/`high_sec_per_km`, but swimming is prescribed in **seconds per 100m** (and `athlete_config` already stores `threshold_swim_pace_sec_per_100m`). The garmin-bridge's pace conversion assumes `/km`. So a "Race Pace" swim cannot carry any pace target — for a triathlete, a third of race day has no prescribable intensity. This is the deferred sibling of `resolve-zone-targets`.

## What Changes

- Add a swim-pace target representation in `workouttemplates.Target`: a distinct `swim_pace` kind with `low_sec_per_100m`/`high_sec_per_100m` fields (self-describing units, no sport context needed to interpret — mirrors how `athlete_config` separates km vs 100m pace).
- Validate `swim_pace` at the template service layer (positive bounds, `low ≤ high`); restrict it to swim workouts.
- Teach the garmin-bridge `workout_builder` to convert `swim_pace` (sec/100m → m/s via `100 / sec_per_100m`) and emit a Garmin pace target.
- Accept `swim_pace` in slot `target_overrides` (so swim pace can progress across weeks like run pace does).
- Out of scope: resolving swim pace from `athlete_config` threshold (no zone math for swim here — direct absolute values, like `pace` today); cadence/secondary/multisport follow-ups.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workout-templates`: the validated step program gains a `swim_pace` target kind (sec/100m), validated and swim-restricted.
- `training-plan`: slot `target_overrides` and the effective program accept and carry `swim_pace` targets.
- `garmin-bridge`: the workout builder converts `swim_pace` (sec/100m) to Garmin's m/s pace target.

## Impact

- **Code**: `internal/workouttemplates/` (Target field + validation), `internal/trainingplan/` (override passthrough — already shape-agnostic), `internal/agenttools/registry_workouttemplates.go` + `registry_trainingplan.go` (tool arg schema/docs), `apps/garmin-bridge/garmin_bridge/workout_builder.py` (`swim_pace` conversion).
- **Docs**: `task swag` for the Target shape change.
- **No migration** — templates are JSON-stored; new field is additive.
- **Relation**: independent of `resolve-zone-targets`; can land before or after. The `pace` (`/km`) kind is unchanged.
