# Proposal: add-workout-templates

## Why

The athlete's reusable workout library — the ~40 swim/bike/run/yoga session
definitions that drive the 18-week plan — lives today as a `WORKOUT_DEFS` Python
dict inside `garmin.py`, with each definition flattened to `(sport, name,
duration, description)`. To make the backend the system of record for the
training plan (the Option B decision), that library has to move into the API
first: the plan's day-slots reference templates, and pushing a session to the
Garmin watch needs a *structured* workout (warmup / intervals / target zones /
cooldown), not just a timer. This change adds the template library as a
standalone capability. It is the foundation the plan and the Garmin-scheduling
edge both build on, so it ships first and can be built and tested entirely
offline — no Garmin dependency.

## What Changes

- **New `workout-templates` capability** (`internal/workouttemplates/`), the
  same package shape as every other capability (types / repo / service /
  handlers / tests).
- **A template is a structured, reusable session**: `sport`, `name`, optional
  `description`, optional `estimated_duration_sec`, and an ordered list of
  **steps**. Each step is either a single executable step (an *intent* —
  warmup/active/interval/recovery/rest/cooldown — plus a *duration* by time,
  distance, lap-button, or open, plus an optional *target* expressed as a HR/
  power zone, pace, RPE, or absolute HR/power range) or a **repeat group** (a
  count plus a nested list of single steps, one level deep). Steps are stored as
  a validated **JSONB** column — always read with the template, never queried
  individually.
- **REST surface** (registered in `internal/httpserver`): `POST /workout-templates`
  (create), `GET /workout-templates` (list, optional `?sport=` filter),
  `GET /workout-templates/{id}`, `PATCH /workout-templates/{id}`,
  `DELETE /workout-templates/{id}`. Idempotency middleware applies to `POST` as
  usual; `PATCH` is partial-update.
- **MCP tools** mirroring the REST surface 1:1 (`create_workout_template`,
  `list_workout_templates`, `get_workout_template`, `patch_workout_template`,
  `delete_workout_template`), each one HTTP call, body forwarded verbatim.
- **Migration `030_add_workout_templates`** — `workout_templates` table with the
  steps JSONB column and a `sport` CHECK reusing the `workouts` sport vocabulary.
- **Docs**: `task swag` regen; README REST + MCP tables gain the new endpoints/
  tools; the MCP integration expected-tools list is bumped.

This change introduces **no** Garmin coupling and **no** changes to `workouts`,
`races`, or `training-phases` — those links are added by the dependent
`add-training-plan` change.

## Capabilities

### New Capabilities

- `workout-templates`: a library of reusable, structured workout definitions
  (sport + named steps with durations and target zones) that the training plan
  references and the Garmin-scheduling edge compiles into watch workouts.

### Modified Capabilities

<!-- None. workouts/training-phases links are added by add-training-plan. -->

## Impact

- **New code**: `internal/workouttemplates/` (types/repo/service/handlers +
  tests), wiring in `internal/httpserver/server.go`, MCP tool group in
  `internal/mcpserver/server.go`, migration `030`.
- **APIs**: five new REST endpoints, five new MCP tools.
- **No breaking changes**: purely additive; no existing endpoint or table is
  touched.
- **Downstream**: unblocks `add-training-plan` (slots reference templates) and
  `add-garmin-scheduling` (steps compile to Garmin structured workouts).
