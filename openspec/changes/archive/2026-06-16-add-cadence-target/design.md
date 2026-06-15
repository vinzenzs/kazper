## Context

`workouttemplates.Target` carries `Low`/`High` ints documented as "in their own units" (used today by `hr_bpm`/`power_w`). The garmin-bridge `_TARGET_TYPE` map translates each kind to a Garmin `workoutTargetType`; absolute kinds emit `targetValueOne`/`targetValueTwo`. There is no cadence kind. Garmin's cadence target is an absolute rpm/spm range (target type id 3), interpreted by the watch as rpm (bike) or spm (run) per the workout/segment sport.

## Goals / Non-Goals

**Goals:**
- A bike or run step can prescribe a cadence range, primary or (with `add-secondary-target`) secondary.

**Non-Goals:**
- Cadence zones (Garmin's cadence target is absolute, not zoned).
- Resolving cadence from any config (no cadence threshold exists).
- Cadence on swim/strength/yoga/etc.

## Decisions

### D1: Reuse `Target.Low`/`High`, new kind `cadence`
No new fields — `cadence` uses the existing `Low`/`High` ints as the range in the sport's native unit (rpm/spm). Mirrors how `hr_bpm`/`power_w` already use `Low`/`High`. Avoids growing the Target struct for a value type it already fits.

### D2: Bike/run only, validated at the template service layer
`cadence` SHALL be accepted only on bike or run templates/steps (sport known at validation); positive bounds, `low <= high`. Swim/strength/etc. reject it.

### D3: Bridge emits Garmin cadence target (type id 3)
Add `"cadence": (3, "cadence")` to `_TARGET_TYPE` and emit `targetValueOne=low`, `targetValueTwo=high` (same branch as `hr_bpm`/`power_w`). Verify whether Garmin needs a sport-specific cadence target key (`cadence` vs `run.cadence`); if so, select it from the segment/workout sport — the builder already has `sport_obj` in scope.

### D4: Compose with secondary-target as its own metric family (gated)
If `add-secondary-target` has shipped, add `cadence` as a distinct metric family in its pair validator so a bike step can pair primary power with secondary cadence (the classic pairing). If not yet shipped, this is a no-op now and the family list gains `cadence` when secondary lands.

## Risks / Trade-offs

- **Sport-specific Garmin cadence key** (`cadence` vs `run.cadence`) → Mitigation: verify against garminconnect during implementation; pass segment sport into the target builder if needed (D3).
- **Unit ambiguity (rpm vs spm)** → Acceptable: the unit is implied by the step's sport, exactly as Garmin does it; D2's bike/run gate keeps it well-defined.

## Migration Plan

Additive JSON value; no DB migration. Deploy code + `task swag`. Rollback = revert; no template uses `cadence` yet.

## Open Questions

- None blocking. (Sport-specific cadence key is a verify-at-implementation detail, not a design fork.)
