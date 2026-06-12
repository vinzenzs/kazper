# Proposal: add-training-plan

## Why

The 18-week triathlon plan — which workout falls on which day of which week —
lives today as markdown tables in `Plan.md`, parsed at runtime by `garmin.py`'s
`_parse_week_schedule`. That makes the plan invisible to the backend: the app
can't show "this week's sessions," the carb-load and race-prep math can't see
upcoming load, and nothing but the Python script can edit it. The Option B
decision is to make the backend the **system of record** for the plan. This
change adds a structured, editable `training-plan` (weeks → day-slots →
templates), anchored to a race and its phases, plus a **materialize** operation
that expands the plan into concrete planned `workouts` rows. After this, the plan
is queryable, per-day editable, and feeds the rest of the system — and `Plan.md`
is retired.

## What Changes

- **New `training-plan` capability** (`internal/trainingplan/`): a plan owns an
  ordered set of **weeks**; each week owns an ordered set of **day-slots**; each
  slot points at a `workout-template` for a given weekday. A plan optionally
  references a `race` (its target) and each week optionally references a
  `training-phase` (for nutrition/intent context).
- **Materialize**: `POST /training-plans/{id}/materialize` expands a scope (a
  single week, a date range, or the whole plan) into planned `workouts` rows —
  one per slot, dated from `plan.start_date + (week.ordinal-1) weeks + weekday`.
  Materialization is **idempotent**, keyed by slot: re-running updates the same
  rows (so editing a slot or shifting `start_date` moves the planned workout
  rather than duplicating it). Multiple slots on one day share a `session_group`
  (the existing brick mechanism).
- **Modified `workouts` capability** (additive): two new nullable columns —
  `template_id` (the template a planned workout came from) and `plan_slot_id`
  (the slot it materializes, with a partial-unique index enabling the
  slot-keyed upsert). A new repo path upserts a planned workout from a slot. No
  existing column, endpoint, or the `external_id` UPSERT path changes; completed
  Garmin activities (which carry `external_id`, never `plan_slot_id`) and
  planned-from-plan rows are disjoint.
- **REST surface**: plan CRUD (`POST/GET/GET{id}/PATCH/DELETE /training-plans`,
  the `GET{id}` returning the nested weeks+slots tree); week sub-resources
  (`POST /training-plans/{id}/weeks`, `PATCH/DELETE …/weeks/{weekId}`); slot
  sub-resources (`POST …/weeks/{weekId}/slots`, `PATCH/DELETE …/slots/{slotId}`);
  and `POST /training-plans/{id}/materialize`.
- **MCP tools** mirroring the surface 1:1 (`create_training_plan`,
  `list_training_plans`, `get_training_plan`, `patch_training_plan`,
  `delete_training_plan`, `add_plan_week`, `patch_plan_week`, `delete_plan_week`,
  `add_plan_slot`, `patch_plan_slot`, `delete_plan_slot`,
  `materialize_training_plan`).
- **Migration `031_add_training_plan`** — `training_plans`, `plan_weeks`,
  `plan_slots` tables; `ALTER workouts ADD template_id, plan_slot_id` + the
  partial-unique slot index.
- **Docs**: `task swag`; README REST + MCP tables; MCP expected-tools list bump.

## Capabilities

### New Capabilities

- `training-plan`: a structured, race-anchored, phase-aware weekly plan
  (weeks → day-slots → templates) plus an idempotent materialize operation that
  expands it into planned `workouts`.

### Modified Capabilities

- `workouts`: gains optional `template_id` and `plan_slot_id` links and a
  slot-keyed upsert path so planned workouts can originate from a plan; the
  `external_id` import path and all existing columns are unchanged.

## Impact

- **Depends on** `add-workout-templates` (slots reference templates).
- **New code**: `internal/trainingplan/`, `workouts` repo/types additions,
  wiring in `internal/httpserver`, MCP tool group, migration `031`.
- **APIs**: plan/week/slot CRUD + materialize REST endpoints; twelve MCP tools.
- **No breaking changes**: all additive. `Plan.md` parsing is retired from the
  workflow (the script can read the plan from the API instead), but that is
  outside this repo.
- **Out of scope**: matching/reconciling a *completed* Garmin activity against
  the *planned* workout it fulfilled (the two rows coexist) — a future change.
