## Context

`workouttemplates.Target` supports `pace` with `LowSecPerKM`/`HighSecPerKM`; the garmin-bridge converts that to m/s via `1000/sec_per_km`. Swimming is prescribed in **sec/100m**, and `athleteconfig.ThresholdSwimPaceSecPer100m` already exists. There is no field, validation, conversion, or test for swim pace; the bridge explicitly does not map swim threshold from Garmin (no reachable source). This change adds the missing target representation only — it does not build zone math for swim.

## Goals / Non-Goals

**Goals:**
- Let a swim step (and a swim slot override) carry an absolute pace target in sec/100m.
- Push that target to the watch as a correct Garmin pace gate.

**Non-Goals:**
- Resolving swim pace from `athlete_config` threshold/zones (no swim power/HR zone math here — direct values, like `pace` today).
- Changing the existing `pace` (`/km`) kind.
- Cadence / secondary / multisport targets (separate backlog items).

## Decisions

### D1: Distinct `swim_pace` kind, not overloaded `pace`
Add `kind:"swim_pace"` with `low_sec_per_100m`/`high_sec_per_100m` rather than reusing `pace` with a unit flag. Self-describing: a target's units are clear without consulting the workout's sport, mirroring `athlete_config`'s `threshold_pace_sec_per_km` vs `threshold_swim_pace_sec_per_100m` split. Alternative (reuse `pace`, interpret `/km` vs `/100m` by sport) rejected — it makes the same `kind` mean two units depending on out-of-band context, which is exactly the ambiguity the rest of the model avoids.

### D2: Swim-restricted at validation
`swim_pace` SHALL be accepted only on swim-sport templates/steps; `pace` (`/km`) SHALL be rejected on swim steps and `swim_pace` rejected on non-swim steps. Validation lives at the template service layer where sport is known. Keeps run/bike using `/km` and swim using `/100m` without silent unit confusion.

### D3: Bridge conversion `100 / sec_per_100m`
Garmin pace targets are m/s. For `swim_pace`, emit `targetValueOne = 100/low_sec_per_100m`, `targetValueTwo = 100/high_sec_per_100m`, using the same Garmin pace target type the `pace` kind already uses. Parallel to the existing `_pace_mps(sec_per_km) = 1000/sec_per_km`.

### D4: Overrides are shape-agnostic — no resolver change
Slot `target_overrides` already carry a full `Target` verbatim and `EffectiveProgram`/`applyOverrides` copy it unchanged. `swim_pace` flows through with no training-plan code change beyond the tool arg schema/docs. If `resolve-zone-targets` has shipped, its resolver passes `swim_pace` through (it is not a zone kind).

## Risks / Trade-offs

- **Two pace kinds to keep straight** → Mitigation: D2 validation makes the wrong kind on the wrong sport a hard error, so they can't be silently swapped.
- **Garmin swim pool/open-water nuances** (pool length, lap semantics) → Out of scope; we emit a pace target and let the watch apply it. Revisit if pool-length handling proves necessary.

## Migration Plan

Additive JSON field; no DB migration. Deploy code + `task swag`. Rollback = revert; existing templates (none use `swim_pace` yet) are unaffected.

## Open Questions

- Should a future change resolve `swim_pace` from `threshold_swim_pace_sec_per_100m` (e.g. "swim threshold ±x")? Deferred — this change takes absolute values only.
