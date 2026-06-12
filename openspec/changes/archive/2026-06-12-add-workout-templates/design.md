# Design: add-workout-templates

## Context

`garmin.py` holds the athlete's workout library as `WORKOUT_DEFS` — a dict of
`key -> (sport, name, duration_seconds, description)`. That flat shape is enough
to *name* a session on a calendar but not to *guide* one on a watch. The Option B
decision makes the backend the system of record for the whole training plan, and
the plan's day-slots need something to point at, so the library moves into the
API as a first-class capability. We also chose the richer template shape
(structured steps, not flat metadata) so a scheduled session arrives on the watch
as a real guided workout (warmup, intervals with target zones, cooldown).

This is the foundation change: it has no dependency on Garmin, on `workouts`, or
on the plan. It can be built and tested entirely offline.

## Goals / Non-Goals

**Goals:**
- A reusable, structured workout-template library following the standard
  per-capability package shape.
- A step model expressive enough to compile to a Garmin structured workout, yet
  clean enough that Garmin's payload shape never leaks into the backend.
- Standard REST + MCP surface, idempotency, swag docs.

**Non-Goals:**
- No link to `workouts`, `training-phases`, or `races` (that is
  `add-training-plan`).
- No Garmin translation here (that is `add-garmin-scheduling`; the bridge owns
  the step → garminconnect payload mapping).
- No performance/streams modelling, no per-step GPS, no swim stroke taxonomy
  (extensible later via the step JSON).

## Decisions

### D1: Steps as a validated JSONB column, not a child table

A template's steps are always read and written as a unit, never queried
individually, and they nest (a repeat group contains steps). A normalized child
table would force joins and ordering bookkeeping for zero query benefit. So
`steps` is a single `JSONB NOT NULL` column; the **service layer** validates
its structure on write (the DB only enforces `jsonb` + non-empty via a CHECK).
This mirrors how the project keeps related-but-read-as-a-unit data together
rather than over-normalizing.

### D2: The step model

Two node kinds, one level of nesting (a repeat may contain single steps, but not
another repeat — matching what the Garmin payload and a human plan actually use):

```jsonc
// steps: ordered array of nodes
// --- single executable step ---
{
  "type": "step",
  "intent": "warmup" | "active" | "interval" | "recovery" | "rest" | "cooldown",
  "duration": { "kind": "time",   "seconds": 600 }   // OR
            // { "kind": "distance", "meters": 400 } OR
            // { "kind": "lap_button" }              OR
            // { "kind": "open" },
  "target": { "kind": "none" }                        // OR
          // { "kind": "hr_zone",   "low": 1, "high": 2 }  OR
          // { "kind": "power_zone", "low": 4, "high": 4 } OR
          // { "kind": "pace",      "low_sec_per_km": 300, "high_sec_per_km": 330 } OR
          // { "kind": "hr_bpm",    "low": 140, "high": 155 } OR
          // { "kind": "power_w",   "low": 200, "high": 240 } OR
          // { "kind": "rpe",       "low": 6,   "high": 7 },
  "note": "catch, pull, rotation"        // optional, free text
}
// --- repeat group ---
{
  "type": "repeat",
  "count": 5,
  "steps": [ { "type": "step", ... }, { "type": "step", ... } ]   // single steps only
}
```

**Validation (service layer), each a sentinel error → 1:1 API code:**
- `steps` is non-empty.
- every node has a recognized `type`; `step` nodes have a valid `intent`,
  exactly one `duration` of a known `kind` (time `seconds > 0`; distance
  `meters > 0`), and a `target` of a known `kind`.
- `target` ranges: zones in `1..5`; when both bounds present `low <= high`;
  pace/HR/power bounds `> 0`.
- `repeat` nodes have `count >= 2`, a non-empty `steps` array, and contain **no**
  nested `repeat` (single-level).
- `sport` is one of the `workouts` sport values (`run|bike|swim|strength|other`)
  — reused deliberately so a template's sport and a workout's sport share one
  vocabulary.

This model is a clean superset of garminconnect's `executableStepDTO`
(endCondition time/distance/lap.button/no.end, target heartrate/power/pace/
no.target) and `repeatGroupDTO` (numberOfIterations + nested steps). The mapping
in both directions lives in the bridge (`add-garmin-scheduling`), not here.

### D3: `estimated_duration_sec` is stored, not derived

`WORKOUT_DEFS` carries an explicit duration per session (e.g. `1800`), and
open/lap-button steps make a computed total unreliable. So the template keeps an
optional author-supplied `estimated_duration_sec` for display and for the plan's
weekly-load rough math, independent of the step durations. It is advisory, not a
sum constraint.

### D4: Table shape (`030_add_workout_templates`)

```
workout_templates(
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sport         TEXT NOT NULL CHECK (sport IN ('run','bike','swim','strength','other')),
  name          TEXT NOT NULL,
  description   TEXT NULL,
  estimated_duration_sec INTEGER NULL CHECK (estimated_duration_sec IS NULL OR estimated_duration_sec > 0),
  steps         JSONB NOT NULL CHECK (jsonb_typeof(steps) = 'array' AND jsonb_array_length(steps) > 0),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
)
-- index on (sport) for the ?sport= list filter
```

No `external_id` / `source` — templates are first-party authored, not imported.
No uniqueness on `name` — the same session name can legitimately recur with
different step detail across plan phases.

### D5: PATCH semantics

`PATCH /workout-templates/{id}` is partial: any of `name`, `description`,
`estimated_duration_sec`, `sport`, `steps` may be supplied; omitted fields are
left unchanged. `steps`, when present, **replaces** the whole array (steps are a
unit; there is no per-step patching). `description`/`estimated_duration_sec`
follow the project's empty-clears convention only if needed; for v1 a present
`null` clears the nullable, a present value sets it, omission leaves unchanged —
consistent with existing PATCH handlers. No `Idempotency-Key` on `PATCH` is
required (POST carries idempotency; PATCH is naturally re-appliable).

## Risks / Trade-offs

- **JSONB validation drift**: because the DB only checks "non-empty array", a bad
  writer could persist a malformed step. Mitigation: all writes go through the
  service validator; tests assert rejection of each malformed shape.
- **Step model vs Garmin reality**: garminconnect's payload evolves. We isolate
  that risk by keeping translation in the bridge — if Garmin adds a step concept
  we don't model, it is a bridge change, not a schema migration.
- **One-level repeat nesting** can't express pyramid-of-pyramids sessions. That
  is rare in this plan; if needed later, lift the nesting restriction and bump
  the validator — no table change required (JSONB).

## Migration Plan

Additive only. New table `030_add_workout_templates`; no backfill (the library
is authored fresh via the API — a one-time seed of the existing `WORKOUT_DEFS`
can be POSTed by a script, out of band). Down migration drops the table.

## Open Questions

- Should swim steps carry a stroke/pool-length field now or later? Deferred —
  the JSONB step can gain an optional `stroke` without migration when the watch
  push needs it.
