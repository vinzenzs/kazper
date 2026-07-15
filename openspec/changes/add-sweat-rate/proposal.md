## Why

The last old triage item (T2 #6C): sweat rate (ml/hr) anchors hydration planning for long races, and every input is loggable in Kazper — pre/post session weight, in-session fluid (hydration + workout-fuel `quantity_ml`, both workout-linkable), duration — but the arithmetic lives in the athlete's head. One derived read closes it.

## What Changes

- `GET /api/v1/workouts/{id}/sweat-rate?pre_weight_kg=&post_weight_kg=` — the standard field test: `sweat_loss_ml = (pre − post) × 1000 + fluid_ml`, `sweat_rate_ml_per_hr = loss / duration_hr`. Pre/post weights are **explicit required params** (the bodyweight log is daily-grained, not pre/post-session); fluid is summed from the workout's linked hydration and workout-fuel `quantity_ml` entries, itemized in the response, with an optional `fluid_ml_override` for unlogged bottles.
- Duration from the completed workout's elapsed duration; `409 workout_not_completed` otherwise. Param errors 1:1 (`pre_weight_invalid`, `post_weight_invalid`, `fluid_override_invalid`); implausible loss (negative or > 5 L/hr) returns the numbers with `warning: "implausible_result"` rather than refusing.
- New `sweat_rate` MCP tool (read tier, one GET, verbatim).
- Compute-on-read, persists nothing, no migration; lands in `internal/workoutfueling/` (the workout-anchored fueling aggregator — it already composes hydration + workout-fuel over a workout).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `workout-fuel`: 2 ADDED requirements — the sweat-rate read and the MCP tool (the workout-anchored fueling aggregation lives in this spec; the code lands in `internal/workoutfueling/`).

## Impact

- **Code:** `internal/workoutfueling/` pure computation + handler over its existing three repos; MCP + golden additive; `task swag`.
- **Out of scope:** persisting results or a sweat-rate history (re-derive on demand), auto pre/post weights from bodyweight timestamps (daily grain makes it a guess), environment correction (temperature/humidity), urine-loss correction.
