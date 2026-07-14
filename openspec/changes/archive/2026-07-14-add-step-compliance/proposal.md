# Add per-step workout compliance scoring

## Why

The plan-adherence reads answer *did the session happen* and *how did the whole
workout compare* (planned vs actual duration/TSS), but not *how was it executed
inside* — "interval 3 was 20W under target" is invisible today even though every
piece exists: templates carry structured steps with targets, completed Garmin
workouts carry per-lap splits, and `template_id` links the two.

## What Changes

- **New capability `workout-compliance`** — a compute-on-read
  `GET /workouts/{id}/compliance` that scores a completed workout's execution
  against its linked template, step by step:
  - Expands the template's repeat groups into a flat executed-step sequence and
    matches laps (splits) to steps **positionally** — the repo's main flow is
    structured execution (templates compile to Garmin watch workouts, which emit
    one lap per step).
  - Resolves each step's target through the **existing effective-program
    resolution** (`trainingplan.EffectiveProgram`: slot overrides +
    athlete-config zone→absolute rewrite) — no new target math.
  - Scores each step: target actual vs resolved band (power/HR/pace per the
    target kind) with `in_band` / `under` / `over` classification and a signed
    deviation, plus planned-vs-actual duration; aggregates a 0–100 overall
    workout compliance score (planned-duration-weighted).
  - Returns an explicit **unavailable** result (with counts) when the lap count
    doesn't match the expanded step count, and structured 404/409 errors when
    the workout is missing, not completed, has no template link, is multisport,
    or has no splits.
- **New MCP tool `workout_compliance`** mirroring the endpoint 1:1 (one GET,
  body forwarded verbatim), registered in the shared registry with a regenerated
  announced-schema golden.
- **No schema change, no migration** — pure read over existing rows.

Non-goals (deferred): multisport workouts (per-leg splits vs per-segment
programs is its own matching problem), tolerant/heuristic lap matching for
manual-lap or free-ride executions, surfacing compliance in the coach-dashboard
workout detail route (follow-up SPA change once the API shape settles), and any
intent-based step weighting beyond duration weighting.

## Capabilities

### New Capabilities

- `workout-compliance` — per-step execution scoring of a completed workout
  against its linked template. Named to sit beside `workout-stats` /
  `workout-fuel` in the workout-adjacent read family; "compliance" is the
  TrainingPeaks term for exactly this (adherence = did it happen, compliance =
  was it executed as written).

### Modified Capabilities

_None._ The `workouts`, `workout-templates`, `training-plan`, and
`athlete-config` requirements are consumed unchanged; the MCP tool requirement
lives inside the new capability's spec (per the `workout-templates` precedent).

## Impact

- **Code:** new `internal/workoutcompliance/` package (types / service /
  handlers / tests — no repo of its own; it reads via the workouts repo and the
  training-plan service). Wiring in `internal/httpserver/server.go` (route
  registered on `/workouts/:id/compliance`, same pattern as `workoutfueling`).
  New registry entry in `internal/agenttools/`.
- **MCP:** +1 tool; `AnnouncedToolNames` derives from the registry (no manual
  list bump), but `internal/mcpserver/testdata/announced_schemas.json` needs a
  `goldengen` regeneration for the new tool's schema.
- **DB:** none — no migration.
- **Docs:** `task swag` regenerates `docs/` for the new endpoint.
