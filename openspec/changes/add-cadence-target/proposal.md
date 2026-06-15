## Why

Garmin offers a **Cadence** step target on both bike (rpm) and run (spm) — used for cadence drills and high-cadence intervals, and as the classic *secondary* pairing with power. We don't model it, so cadence-based prescriptions are impossible and the power+cadence secondary target (`add-secondary-target`) has nothing to pair power with.

## What Changes

- Add a `cadence` target `kind` reusing the existing `Target.Low`/`Target.High` int fields (the range in the sport's native unit — rpm for bike, spm for run).
- Validate `cadence`: positive bounds, `low <= high`; accepted only on **bike** and **run** templates/steps.
- Teach the garmin-bridge `workout_builder` to emit a Garmin cadence target (target type id 3, `cadence`) from `low`/`high`.
- `cadence` flows through slot `target_overrides` and the effective program unchanged (shape-agnostic).
- **Coupling with `add-secondary-target`** (gated on its state): register `cadence` as its own metric family so a bike step can pair primary power with secondary cadence.
- Out of scope: cadence zones (Garmin's "cadence" target is an absolute range, not a zone); resolving cadence from any config; non-run/bike sports.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workout-templates`: the validated step program gains a `cadence` target kind (bike/run only).
- `garmin-bridge`: the workout builder emits a Garmin cadence target from `low`/`high`.

## Impact

- **Code**: `internal/workouttemplates/` (kind + validation), `internal/agenttools/registry_workouttemplates.go` + `registry_trainingplan.go` (tool arg docs), `apps/garmin-bridge/garmin_bridge/workout_builder.py` (`cadence` entry in `_TARGET_TYPE` + emission). `internal/trainingplan/` carries it through unchanged.
- **Docs**: `task swag` for the Target shape (new kind).
- **No migration** — targets are JSON-stored; additive.
- **Relation**: independent and small; pairs with `add-secondary-target` (gated family registration). The existing kinds are unchanged.
