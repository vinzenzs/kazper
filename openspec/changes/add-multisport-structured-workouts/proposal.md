## Why

A triathlon or brick is one continuous session — swim → T1 → bike → T2 → run — that should push to the watch as a **single multisport workout** that auto-advances through its segments. Today every workout and template is strictly single-sport; the model fakes a brick only as separate planned-workout rows sharing a `session_group`, and the garmin-bridge builder emits exactly one segment. So the athlete cannot get a real structured triathlon/brick on the watch with transitions. This is the largest of the workout-target follow-ups and the one that finally requires per-segment sport.

## What Changes

- Introduce a **multisport workout template**: an ordered list of **segments**, each segment carrying its own `sport` and its own step program (the existing single-step / repeat-group model), plus optional **transition** segments (T1/T2).
- Validate segments: ≥2 non-transition segments, each segment's steps validated by the existing step validator under that segment's sport (so per-segment rules apply — e.g. swim uses `swim_pace`, bike may carry `secondary_target`).
- Teach the garmin-bridge `workout_builder` to emit a multi-segment payload: one `workoutSegments` entry per segment with its own `sportType`, monotonic `segmentOrder`, and a transition sport type for T1/T2.
- Add a REST + MCP surface to create/read multisport templates and a **schedule** action that compiles one to a Garmin calendar date (reusing the existing bridge calendar mechanics).
- **Per-segment composition with existing changes** (gated on their state): the `resolve-zone-targets` zone resolver and the `add-secondary-target` bike-only rules apply **per segment sport** — the bike segment resolves `power_zone`/may carry a secondary; the run/swim segments do not.
- **Deferred to a later phase (out of scope here)**: full training-plan integration — a plan slot referencing a multisport template, materialize emitting a multisport planned workout, and a `multisport`/`transition` workouts-table sport. The `workouts` row and `Program` type stay single-sport this phase; multisport lives in its own library + direct-schedule path. The existing brick-via-`session_group` ingestion of completed activities is unchanged.

## Capabilities

### New Capabilities
- `multisport-workouts`: the segmented multisport template model, its validation, REST/MCP CRUD, and compile-and-schedule-to-Garmin action.

### Modified Capabilities
- `garmin-bridge`: the workout builder gains a multi-segment payload path (multiple `workoutSegments`, per-segment sport, transition segments).

## Impact

- **Code**: new `internal/multisport/` (types, repo, service, handlers) reusing `workouttemplates.Step` validation; `internal/httpserver/server.go` wiring; `internal/agenttools/` new tool group; `apps/garmin-bridge/garmin_bridge/workout_builder.py` (multi-segment + transition sport id); `internal/garmincontrol/` schedule path for a multisport template.
- **Migration**: a new table for multisport templates (segments stored as JSONB). First new migration since head `041` — verify the next number before scaffolding.
- **Docs**: `task swag`; MCP integration expected-tools list bumped for the new tools.
- **Ordering / coupling**: extends the step contract that `resolve-zone-targets` and `add-secondary-target` operate on. Land those first so the resolver and secondary rules are applied per-segment in one place. Phase 2 (plan integration) is a separate later proposal.
