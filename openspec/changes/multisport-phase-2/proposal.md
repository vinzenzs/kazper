## Why

Phase 1 (`add-multisport-structured-workouts`) shipped multisport **templates** with their own library, validation, and a direct compile-and-schedule-to-Garmin action — but they live entirely outside the training plan. A triathlon or brick still can't sit on a plan day: the plan's `weeks → slots → materialize` pipeline only references single-sport `workout_templates`, materialize only emits single-sport `workouts` rows, and `Program`/`EffectiveProgram` (the chokepoint that feeds both `get_workout_program` and the Garmin push) is single-sport. So the athlete must schedule each brick by hand, off-plan, and it never shows up in the materialized week. Phase 2 closes that gap: a plan slot can reference a multisport template, materialize emits a multisport planned workout, and the existing effective-program → push path carries it to the watch.

## What Changes

- A **plan slot** may reference **either** a single-sport `workout_templates` row **or** a `multisport_templates` row (exactly one). The slot gains a nullable `multisport_template_id` alongside the existing `template_id`; a slot carrying both, or neither, is rejected.
- **Materialize** emits a multisport planned **workout** row for a multisport slot: `sport = "multisport"`, referencing the multisport template, with the same date/time placement as a single-sport slot.
- The `workouts` sport vocabulary gains **`multisport`** (the planned/pushed brick row). `transition` does **not** become a workout sport — it stays a per-segment sport inside a multisport template.
- **`EffectiveProgram`** returns a multisport program when the workout is multisport: a `Program` gains an optional ordered list of **segments** (each its own sport + resolved steps), populated from the multisport template; single-sport workouts are unchanged (`Steps`, empty segments).
- **Per-segment target resolution** (gated on `resolve-zone-targets`, shipped): the zone resolver runs over each segment keyed by **that segment's sport** — the bike segment resolves `power_zone`, the run/swim segments pass through — finally delivering the per-segment-sport awareness the single-sport resolver explicitly deferred. Bike segments may carry `secondary_target` (gated on `add-secondary-target`, shipped).
- The **Garmin push path** (`pushOne → EffectiveProgram → bridgeCreateWorkout`) sends the multisport form for a multisport workout, reusing Phase 1's bridge multi-segment compile. `get_workout_program` returns the segments for display.
- **REST + MCP**: the existing slot create/patch surface accepts a `multisport_template_id`; `get_workout_program` surfaces segments. No new tools (the multisport-template CRUD tools already exist).

**Out of scope (this phase):** per-intent slot **target/duration overrides** for multisport slots (intent collides across segment sports — multisport slots materialize the template as-authored); changing completed-brick ingestion (the `session_group` decomposition of *completed* activities is unchanged); open-water/pool nuances.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `training-plan`: a plan slot may reference a multisport template (XOR with the single-sport template); materialize emits a `multisport` planned workout; the effective program returns per-sport segments (resolved per segment sport) for a multisport workout.
- `workouts`: the sport vocabulary gains `multisport`; a workout row may carry a `multisport_template_id` instead of a single-sport `template_id`.
- `multisport-workouts`: a multisport template is no longer schedule-only — it can be referenced by a plan slot and reach the watch through the plan's materialize + effective-program + push path, not only the direct schedule action.

## Impact

- **Code**: `internal/trainingplan/` (slot XOR template ref, materialize branch, `EffectiveProgram` segments + per-segment resolver, new `multisport` repo cross-injection), `internal/workouts/` (`multisport` sport, `multisport_template_id` column + planned-from-slot input), `internal/garmincontrol/` (push path emits the multisport form via `EffectiveProgram`), `internal/agenttools/` (slot tool args gain `multisport_template_id`), `internal/httpserver/server.go` wiring.
- **Migration**: `046` — add `multisport_template_id` to `plan_slots` and to `workouts` (nullable, FK to `multisport_templates`), and widen the `workouts.sport` CHECK to include `multisport`. First migration after head `045`.
- **Docs**: `task swag`; the MCP expected-tools list is unchanged (no new tools — only arg schemas grow, so the golden baseline is regenerated as in the swim_pace/cadence precedent).
- **Coupling**: builds directly on Phase 1's `multisport-workouts` capability and bridge multi-segment compile, and on the shipped `resolve-zone-targets` / `add-secondary-target` (applied per segment). No garmin-bridge change.
