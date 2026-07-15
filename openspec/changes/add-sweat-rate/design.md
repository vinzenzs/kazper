## Context

`internal/workoutfueling/` is the aggregation-only package composing hydration + workout-fuel + workouts (it exists precisely because the first two import `workouts` — the import-cycle note in CLAUDE.md), serving `/workouts/{id}/fueling`. Sweat rate is the same composition plus two weights and a division: `loss_ml = Δweight_kg × 1000 + fluid_ml`, rate over elapsed hours.

## Goals / Non-Goals

**Goals:** the standard field-test arithmetic with every input visible (itemized fluid, echoed params); explicit-params honesty where Kazper's data is too coarse (daily weight ≠ pre/post weight).

**Non-Goals:** persisting results/history, auto-weights from the bodyweight log, environmental or urine corrections, hydration-plan *recommendations* (the coach's job, fed by this number).

## Decisions

### D1 — Pre/post weights are explicit required params
The bodyweight capability is daily-grained; inferring pre/post from it would be a guess dressed as data. The W′bal params convention: caller supplies, response echoes. `pre_weight_invalid`/`post_weight_invalid` on missing/non-positive; plausibility bounds only warn (D4).

### D2 — Fluid = linked hydration ml + linked workout-fuel `quantity_ml`, itemized
Summed from entries carrying this `workout_id`; the response itemizes `{hydration_ml, workout_fuel_ml, fluid_ml_override?}` so a surprising rate is auditable. `fluid_ml_override` (≥ 0) replaces the derived sum entirely when supplied — for the unlogged-bottle case — and its use is echoed.

### D3 — Duration from the completed workout's elapsed duration
`409 workout_not_completed` for planned workouts; elapsed (not moving) time matches how the field test is defined. No duration override — if the duration is wrong, fix the workout.

### D4 — Warn, don't refuse, on implausible results
Negative loss (weight gain) or > 5 L/hr returns the computed numbers plus `warning: "implausible_result"` — the inputs are athlete-supplied, and showing the arithmetic beats hiding it; refusing would just force a recalculation elsewhere. Values `Round1` (ml and ml/hr as whole-ish numbers, 1 dp), boundary-only.

### D5 — Unit isolation holds
The response is ml/ml-per-hr/kg only — no kcal, no sodium (sodium planning reads the existing fueling summary), nothing feeding nutrition or hydration daily totals.

## Risks / Trade-offs

- **Garbage weights → confident number** — mitigated by the echo + warning band; it's a calculator with provenance, framed as such in the tool description.
- **Fluid double-log risk** (same bottle in hydration and workout-fuel) — pre-existing data-hygiene concern; itemization makes it visible here.

## Migration Plan

None. Rollback = revert route/tool.

## Open Questions

- A sweat-rate *history* once several field tests exist (needs persistence — deliberately deferred).
