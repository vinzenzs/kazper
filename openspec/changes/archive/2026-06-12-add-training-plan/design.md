# Design: add-training-plan

## Context

`Plan.md` encodes the 18-week plan as `### Week N` markdown tables of
`weekday → workout_key`, parsed at runtime by `garmin.py`. Option B moves that
structure into the backend so the plan is queryable, editable per-day, and
visible to the app and the fueling math. The plan's skeleton already exists —
`races` (race day) and `training-phases` (date-ranged, with default goal
templates and nutrition ranges) — and `workouts` already has `status='planned'`.
So this change adds the missing middle: the **recurring weekly structure** and
the **materialize** step that turns it into dated planned workouts.

## Goals / Non-Goals

**Goals:**
- A structured, editable plan: plan → weeks → slots → template.
- Anchor to a `race` and per-week `training-phase` for intent/nutrition context.
- An idempotent `materialize` that expands the plan into planned `workouts`,
  re-runnable without duplication.
- Reuse existing primitives (`workouts.status=planned`, `session_group`,
  `training-phases`, `races`) rather than re-modelling them.

**Non-Goals:**
- No Garmin coupling (that is `add-garmin-scheduling`, which reads the
  materialized planned workouts).
- No reconciliation of completed activities against planned workouts (future).
- No recurrence rules beyond an explicit per-week, per-weekday slot list — the
  plan is enumerated, not generated from a rule.

## Decisions

### D1: Three tables — plan / week / slot — with stable slot ids

```
training_plans(
  id UUID PK, name TEXT NOT NULL,
  race_id UUID NULL REFERENCES races(id) ON DELETE SET NULL,
  start_date DATE NOT NULL,            -- the Monday of week 1
  notes TEXT NULL, created_at, updated_at )

plan_weeks(
  id UUID PK,
  plan_id UUID NOT NULL REFERENCES training_plans(id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL CHECK (ordinal >= 1),   -- week 1..N
  phase_id UUID NULL REFERENCES training_phases(id) ON DELETE SET NULL,
  notes TEXT NULL, created_at, updated_at,
  UNIQUE (plan_id, ordinal) )

plan_slots(
  id UUID PK,
  plan_week_id UUID NOT NULL REFERENCES plan_weeks(id) ON DELETE CASCADE,
  weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),   -- 0=Mon … 6=Sun
  ordinal SMALLINT NOT NULL DEFAULT 0,            -- order of sessions within a day
  template_id UUID NOT NULL REFERENCES workout_templates(id) ON DELETE RESTRICT,
  time_of_day TIME NULL,                          -- optional; default applied at materialize
  created_at, updated_at )
```

Slots are managed via their own sub-resource endpoints (not replaced wholesale by
a plan PUT) **specifically so slot ids are stable** — the materialize upsert is
keyed on `plan_slot_id`, so a stable id is what lets re-materialize update rather
than duplicate. `ON DELETE RESTRICT` on `template_id` prevents deleting a
template still referenced by a plan.

### D2: `workouts` gains `template_id` + `plan_slot_id` (the modification)

```
ALTER TABLE workouts
  ADD COLUMN template_id  UUID NULL REFERENCES workout_templates(id) ON DELETE SET NULL,
  ADD COLUMN plan_slot_id UUID NULL REFERENCES plan_slots(id)        ON DELETE SET NULL;
CREATE UNIQUE INDEX workouts_plan_slot_id_key
  ON workouts (plan_slot_id) WHERE plan_slot_id IS NOT NULL;
```

A planned-from-plan workout carries both links; an imported Garmin activity
carries `external_id` and neither plan link — the two populations are disjoint,
so the new slot-keyed upsert and the existing `external_id` upsert never collide.
`ON DELETE SET NULL` means deleting a slot/template detaches but preserves the
workout row (history is never destroyed by a structural edit). A separate
"unmaterialize" is out of scope; deleting planned rows is an ordinary
`DELETE /workouts/{id}`.

### D3: Materialize — deterministic expansion, slot-keyed upsert

`POST /training-plans/{id}/materialize` with a scope:

```jsonc
{ "scope": "week",  "week": 5 }                       // one week
{ "scope": "range", "from": "2026-06-15", "to": "2026-06-28" }  // by date
{ "scope": "all" }                                    // the whole plan
```

For each in-scope week and each slot:

```
date       = plan.start_date + (week.ordinal - 1)*7 days + slot.weekday
start_time = slot.time_of_day, or a default (06:00 local) stacked by slot.ordinal
started_at = date @ start_time
ended_at   = started_at + (template.estimated_duration_sec or 3600s)
UPSERT workouts (plan_slot_id=slot.id) DO UPDATE SET
  sport=template.sport, name=template.name, status='planned',
  template_id=template.id, started_at, ended_at,
  session_group = <group key for (week,weekday)> when the day has >1 slot
WHERE workouts.status = 'planned'   -- never resurrect a fulfilled session
```

**The `WHERE status='planned'` guard is load-bearing.** A future
reconciliation change lets a Garmin import *fulfill* a planned workout by
flipping it `planned → completed` in place (keeping its `plan_slot_id`). Without
the guard, re-materializing the plan would match that fulfilled row by
`plan_slot_id` and clobber it back to `planned`, wiping the actuals. The guard
makes materialize idempotently skip any slot whose workout is already completed,
so the plan-materialize and reconciliation paths compose safely. Until
reconciliation ships, no `plan_slot_id` row is ever completed, so the guard is a
harmless no-op — cheap insurance bought now.

Keyed on `plan_slot_id`, so:
- re-materializing is idempotent (same rows updated);
- editing a slot's template/weekday/time and re-materializing **moves/retargets**
  the existing planned workout;
- shifting `plan.start_date` and re-materializing **moves all dates** without
  creating duplicates.

Materialize only ever writes rows with `status='planned'` and a `plan_slot_id`;
it never touches a completed activity. The response returns the list of planned
workouts created/updated.

**Brick grouping:** when a (week, weekday) has more than one slot, all its
materialized workouts get the same generated `session_group` value — reusing the
existing brick/multisport mechanism in `workouts`.

### D4: Materialize is a backend primitive, not agent synthesis

Expanding `plan + start_date` into dated rows is deterministic arithmetic with a
correctness-critical idempotency key — exactly the kind of primitive the API
should own, not the agent. (The *coaching* synthesis over the resulting plan
stays with the agent.)

### D5: REST surface — nested read, sub-resource writes

`GET /training-plans/{id}` returns the full nested tree (weeks, each with its
slots) for display and for the Garmin-scheduling edge to read. Writes are
per-resource (plan, week, slot) to keep slot ids stable per D1. `materialize` is
a plan-level action endpoint. PATCH is partial on each resource. `POST`s honor
`Idempotency-Key`; `materialize` is naturally idempotent so the header is
optional and re-running is safe.

## Risks / Trade-offs

- **Surface size**: plan + week + slot CRUD + materialize is ~12 endpoints and
  as many MCP tools. That is the cost of making the plan a true system of record;
  the alternative (whole-plan PUT) breaks slot-id stability and the materialize
  idempotency, so it is rejected.
- **Planned vs actual divergence**: after Garmin imports the real activity, the
  planned workout and the completed one coexist as two rows. Reconciliation is
  deliberately deferred; until then, downstream consumers distinguish them by
  `status` and `plan_slot_id`/`external_id`.
- **Default start time** (06:00) is a guess when `time_of_day` is null. It only
  affects the planned window, not the day it lands on; the athlete can set
  `time_of_day` per slot when it matters (e.g. for the Garmin calendar).

## Migration Plan

`031_add_training_plan`: create the three tables, then `ALTER workouts` to add
the two columns and the partial-unique slot index. Additive; no backfill. Down
migration drops the index, the two columns, then the three tables (reverse FK
order).

## Open Questions

- Should a plan be unique per race (one active plan), or many? v1 allows many
  plans; no uniqueness on `race_id`. Revisit if the app needs a "current plan."
- Reconciliation of completed↔planned (matching a Garmin import to the slot it
  fulfilled) — separate future change; noted as a non-goal here.
