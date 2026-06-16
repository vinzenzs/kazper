## Context

Phase 1 (`add-multisport-structured-workouts`, archived 2026-06-16) delivered the `multisport-workouts` capability: a `multisport_templates` table (segments as JSONB), per-segment-sport validation reusing `workouttemplates.Step`, REST+MCP CRUD, and a direct `POST /garmin/schedule/multisport` action that compiles one template to a Garmin calendar date via the bridge's multi-segment payload path.

What it deliberately deferred is **plan integration**. The training plan's pipeline is single-sport end to end:
- `plan_slots.template_id` is a non-null FK to `workout_templates` (`internal/trainingplan/types.go:51`).
- `Materialize` (`service.go:352`) calls `workoutsRepo.UpsertPlannedFromSlot` with `Sport: sl.TemplateSport`, creating a single-sport `workouts` row.
- `EffectiveProgram` (`service.go:457`) loads the single template, applies overrides, runs the zone resolver, and returns a `Program{Sport, Steps}` — the single chokepoint for both `get_workout_program` and `garmincontrol.pushOne`.
- `workouts.sport` CHECK is `('run','bike','swim','strength','yoga','mobility','other')` (migration `044`); there is no `multisport`.

Phase 1's design D6 spelled out the seam Phase 2 lands on: the zone resolver should run **per segment, keyed by the segment's sport** — the per-segment awareness the single-sport `resolve-zone-targets` (D7) explicitly skipped. Migration head is `045`.

## Goals / Non-Goals

**Goals:**
- A plan slot can schedule a multisport template; `Materialize` emits a `multisport` planned workout; the existing effective-program → push path carries it to the watch with no new push code.
- `EffectiveProgram` is still the single representation both consumers build from — extended, not duplicated.
- Per-segment target resolution: bike segment resolves `power_zone`, run/swim pass through; bike segments may carry `secondary_target`.
- Single-sport behavior is byte-for-byte unchanged.

**Non-Goals:**
- Per-intent **target/duration overrides** on a multisport slot (intent is ambiguous across segment sports). A multisport slot materializes the template as authored.
- Changing completed-activity ingestion — the `session_group` decomposition of *completed* bricks stays.
- A `transition` value in `workouts.Sport` (transition is segment-scoped only).
- Building a multisport template from existing single-sport template ids (Phase 1 open question; still deferred).

## Decisions

### D1: Slot references a template via XOR nullable columns
`plan_slots` gains a nullable `multisport_template_id` (FK → `multisport_templates`), and `template_id` becomes **nullable**. The service enforces **exactly one** is set (`ErrTemplateRequired` when neither; a new `ErrTemplateAmbiguous` / `override_target_invalid`-style sentinel when both). Chosen over a `template_kind` discriminator + single polymorphic id (loses FK integrity) and over a separate slot table (duplicates all slot plumbing). A DB CHECK enforces the XOR as a backstop.

### D2: `workouts` gains a `multisport` sport + `multisport_template_id`
Widen the `workouts.sport` CHECK to add `multisport`, and add a nullable `multisport_template_id` FK. A materialized multisport row has `sport='multisport'`, `template_id=NULL`, `multisport_template_id=<id>`. `transition` is **not** added — it only ever appears as a segment sport inside a multisport template, never as a workout row's sport. The mirrored `workouttemplates`/`workouts` sport vocab note stays accurate because `multisport` is a workout-level concept, not a template sport.

### D3: `Program` gains optional `Segments`; `EffectiveProgram` branches on workout sport
`trainingplan.Program` gains `Segments []ProgramSegment` (`omitempty`), each `{sport, steps}`. When `w.Sport == "multisport"`, `EffectiveProgram` loads the multisport template via an injected `multisportRepo` (cross-injected like `athleteConfigRepo`), resolves **each segment's steps keyed by that segment's sport**, and returns `Program{Sport:"multisport", Segments:[…], Steps: nil}`. Single-sport workouts return `Steps` with empty `Segments` exactly as today. No slot overrides are applied to segments (D-non-goal). `trainingplan` already imports `workouttemplates`/`workouts`/`athleteconfig`; adding `multisport` (which imports only `workouttemplates`) introduces no cycle.

### D4: Per-segment resolution reuses the existing resolver
The Phase-1-era `resolveTargets(steps, cfg, sport)` (added by `resolve-zone-targets`) is called once per segment with the segment's sport — so the existing bike-gate (D7) and HR-cross-sport rules apply naturally per leg, and `secondary_target` resolution composes the same way. No new resolver logic; the multisport path is a loop over segments around the same call.

### D5: Push path is unchanged code, new data
`garmincontrol.pushOne` already calls `EffectiveProgram` and forwards `prog` to `bridgeCreateWorkout`. The bridge request builder sends the **multisport form** (segments) when `prog.Segments` is non-empty, which Phase 1's `workout_builder` already compiles to multiple `workoutSegments`. So the push path gains a branch in how it marshals the bridge body, not a new endpoint. `get_workout_program` returns `Segments` for display.

### D6: Materialize idempotency unchanged
`Materialize` keys planned rows by slot as today; a multisport slot upserts one `multisport` workout row. Re-materialize after editing the multisport template re-pushes new segments on next schedule (same live-engine property as single-sport). No snapshotting.

## Risks / Trade-offs

- **`EffectiveProgram` shape change** (`Segments` appears) → Additive `omitempty` field; single-sport responses are unchanged. Consumers that only read `Steps` keep working; `get_workout_program` and the push builder learn `Segments`.
- **A slot's template kind can change on patch** (single ↔ multisport) → The XOR validation runs on every create/patch; a patch that would leave both or neither set is rejected. Re-materialize then changes the planned row's sport.
- **Overrides silently not applied to multisport slots** → Documented non-goal + a validation error if `target_overrides`/`duration_overrides` are supplied on a multisport slot (fail loud, not silent).
- **Golden MCP schema drift** (slot tool args gain `multisport_template_id`) → Regenerate the announced-schema baseline, as the swim_pace/cadence changes did; no new tools, so the expected-tools list is unchanged.

## Migration Plan

Migration `046_add_multisport_plan_integration`:
- `ALTER TABLE plan_slots ALTER COLUMN template_id DROP NOT NULL; ADD COLUMN multisport_template_id UUID NULL REFERENCES multisport_templates(id); ADD CHECK ((template_id IS NULL) <> (multisport_template_id IS NULL))`.
- `ALTER TABLE workouts ADD COLUMN multisport_template_id UUID NULL REFERENCES multisport_templates(id)`; widen the `sport` CHECK to include `'multisport'`.
- Down: drop the columns/constraints and restore `template_id NOT NULL` (safe — no multisport slots exist before this ships).

Rollback = revert the commit + down migration; existing single-sport slots/rows are untouched (their `multisport_template_id` is NULL).

## Open Questions

- Should a multisport planned workout expose an estimated duration (sum of segment durations + transitions) on the `workouts` row, or leave it null until completed? Lean: derive it in materialize like single-sport `estimated_duration_sec`, but defer if segment durations are open/lap-button.
- Whether `get_training_context` / weekly views should special-case `multisport` rows in any aggregation — defer until a consumer needs it.
