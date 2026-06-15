## Context

Garmin's structured-workout payload already has a `workoutSegments` array, but the bridge always emits exactly one segment (`segmentOrder: 1`, single `sportType`). Our `Template`, `workouts.Sport`, and `trainingplan.Program` are all single-sport; bricks exist only as separate planned rows linked by `session_group` (a read/ingestion grouping), and the materialize path already stacks multiple same-day slots under one `session_group`. There is no transition (T1/T2) concept anywhere. This change adds true multisport on the **prescription/push** side without disturbing the single-sport invariant the rest of the system relies on.

## Goals / Non-Goals

**Goals:**
- Model a multisport session as ordered, per-sport segments with transitions.
- Compile and push it to the watch as one auto-advancing Garmin multisport workout.
- Reuse the existing step model and per-sport validation so swim_pace / secondary-target / zone resolution all apply per segment.

**Non-Goals (this phase):**
- A plan slot referencing a multisport template; materialize emitting a multisport planned workout; a `multisport`/`transition` value in `workouts.Sport`. (Phase 2.)
- Changing how completed brick activities are ingested/grouped (`session_group` stays).
- Open-water vs pool nuances beyond what single-sport already does.

## Decisions

### D1: New `multisport-workouts` capability, not an extension of `workout-templates`
Keep single-sport `workout-templates` untouched so the ~45 templates, the resolver, and secondary-target stay stable. A multisport template is a distinct entity in its own package/table. Alternative (add a `segments` field to `Template`) rejected — it makes every single-sport consumer branch on "is this multisport?".

### D2: Inline segments, not compose-by-reference
A multisport template stores its segments inline (`[{sport, steps[], transition?}]`), not references to existing single-sport template ids. Maps 1:1 to Garmin `workoutSegments`, no referential-integrity problem when a referenced template changes underneath, and the compile path is a straight walk. Trade-off: step content can duplicate an existing template. Recorded as an open question (a future "build a multisport template from existing template ids" convenience could populate segments without changing storage).

### D3: Transitions are segments with `sport:"transition"`
Model T1/T2 as a segment kind carrying a sport of `transition` and a duration (`time`/`lap_button`/`open`), no targets. The bridge maps it to Garmin's transition sport type and its own `workoutSegments` entry. Keeps ordering uniform (every position in the session is a segment) rather than a special between-segments object.

### D4: Per-segment validation under the segment's sport
Each segment's steps run through the existing step validator **with that segment's sport** — so swim segments accept `swim_pace` and reject `/km` pace, bike segments may carry `secondary_target`, etc. Require ≥2 non-transition segments (else it's just a single-sport workout). This is the single place per-segment sport enters the model.

### D5: Compile + schedule reuse existing bridge mechanics
The bridge `build_payload` gains a multi-segment branch producing `workoutSegments: [{segmentOrder, sportType, workoutSteps}, …]` with a monotonic `segmentOrder` and the global step-order counter spanning all segments (Garmin numbers every step across the whole workout). Scheduling reuses the existing calendar create/schedule calls — a multisport workout is just another library object with an id.

### D6: Resolver & secondary-target compose per segment (gated)
If `resolve-zone-targets` has shipped, its resolver runs over each segment's steps **keyed by the segment's sport** (bike segment resolves `power_zone`; run/swim pass through) — the per-segment-sport awareness the single-sport version explicitly deferred. If `add-secondary-target` has shipped, bike segments may carry secondary targets and the bridge emits them per segment. Both are tasks gated on those changes existing at apply time.

## Risks / Trade-offs

- **Largest change in the family; broad surface** → Mitigation: phase it — library + compile + direct-schedule now; plan integration later. The single-sport invariant is preserved, so blast radius is contained to the new package + the bridge builder.
- **Garmin segment/transition field exactness** (sport type id for transition, cross-segment step ordering) → Mitigation: verify against garminconnect when implementing; the bridge is the only place these appear.
- **Step-content duplication vs existing templates** → Accepted for now (D2); compose-by-reference is an open question.
- **Phase-2 ripple** (multisport in `workouts.Sport`/`Program`/materialize) → Deliberately deferred; flagged so it isn't assumed done.

## Migration Plan

One new migration: a `multisport_templates` table (segments as JSONB). Verify the next sequential number (head is `041` on disk; out-of-band slots have happened). Additive — no change to existing tables. Rollback = drop the table + revert code; nothing else references it. `task swag` for the new surface; bump the MCP expected-tools list.

## Open Questions

- **Compose-by-reference convenience**: a builder that assembles segments from existing single-sport template ids (snapshotting their steps into the multisport template at creation)? Defer until the inline model is in use.
- **Phase 2 plan integration**: does a multisport session belong as one plan slot (new), or stay outside the plan as a directly-scheduled library object? Decide when Phase 2 is proposed.
- **Transition defaults**: fixed T1/T2 durations vs lap-button? Likely lap-button (athlete-paced); confirm with real use.
