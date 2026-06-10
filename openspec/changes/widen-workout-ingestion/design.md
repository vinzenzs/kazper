# widen-workout-ingestion — design

## Context

The `workouts` table (head migration `018`) stores the minimum nutrition tools needed at the time: sport, time window, `kcal_burned`, `avg_hr`, `tss`, plus the post-session `rpe`/`gi_distress_score` pair. The Garmin Connect data the (planned) importer reads carries more that is directly fueling-relevant: distance, average power, ambient temperature, estimated sweat loss, and multisport/brick leg structure. None of it fits the current shape.

Constraints inherited from the existing capability:

- The shape is **source-agnostic** (the workouts spec is explicit: any writer may target it).
- `POST /workouts` is an external_id UPSERT with **full-replace of the mutable field set** — new fields must join that set.
- `PATCH /workouts/:id` validates against an explicit **mutable-field allowlist** (`handlers.go` probe map) and uses two patterns for nullables: plain set-only pointers (kcal_burned/avg_hr/tss) and JSON-null tri-state with `Clear*` flags (rpe/gi).
- Performance analysis (laps, splits, GPS, streams) is explicitly out of scope for the capability — this change must not creep toward it.
- Response-boundary rounding via `numfmt.Round1` for numeric values.

## Goals / Non-Goals

**Goals:**

- Five nullable columns any writer can populate: `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, `session_group`.
- Full write-path coverage: POST, `/workouts/bulk`, PATCH; full read-path echo: GET by id, list, fueling-block echo for the two fueling-relevant fields.
- A way to retrieve the legs of one brick/multisport session together (`?session_group=` list filter).
- Zero behavior change for rows/writers that don't use the new fields.

**Non-Goals:**

- Consuming the data (recommendation personalization, sweat-rate derivation) — follow-up changes.
- Planned workouts, race entity, NP/IF/cadence/elevation storage, a `brick` sport value (all per proposal).
- Any change to `garmin.py` inside this repo (out-of-repo companion work).

## Decisions

### 1. Five flat nullable columns, not a JSONB `metrics` blob

A JSONB column would absorb future fields without migrations, but it forfeits CHECK constraints, typed scanning, and the explicit-shape discipline every other capability follows (the unit-isolation convention exists precisely because implicit shapes caused footguns). Five columns matching the existing nullable-field pattern is boring and consistent. Migrations are cheap here.

### 2. `session_group` is free-text TEXT, not an FK to a sessions table

Alternatives considered: (a) a `multisport_sessions` parent table with FK; (b) self-referencing `parent_workout_id`; (c) free-text group key. Chose (c):

- (a) invents an entity with no other attributes today — pure ceremony until a race entity exists (explicitly a later change, which can adopt these group keys).
- (b) forces an artificial parent leg (which leg of a brick is "the parent"?) and breaks when the importer sees legs in arbitrary order.
- (c) is writer-friendly: the Garmin importer sets the parent multisport activity's id (e.g. `garmin:12345678`) on every leg; a manual brick gets any agreed string. Idempotent re-syncs are naturally stable. Grouping is recoverable by simple equality — `GET /workouts?from=&to=&session_group=` returns the legs in `started_at` order, which IS the leg order.

Validation: when present, must be non-empty after trimming and ≤ 255 chars (`session_group_invalid`). Stored verbatim otherwise.

### 3. All five fields use the JSON-null tri-state on PATCH (rpe precedent)

The older set-only pointer pattern (kcal_burned) cannot express "clear this value". Clearing matters here: a wrongly grouped leg must be un-groupable (`{"session_group": null}`), and a fat-fingered manual sweat-loss entry must be erasable without delete/re-create (POST-upsert correction only works for rows that have an `external_id`). The rpe/gi tri-state (`json.RawMessage`, `"null"` → `Clear*` flag) is the established newer convention — apply it uniformly to all five fields rather than mixing patterns within one change. `PatchInput` gains five value pointers + five `Clear*` bools.

The existing kcal_burned/avg_hr/tss set-only semantics are NOT retrofitted — out of scope, and changing them silently would alter the API contract of fields already in use.

### 4. Units: metres, watts, °C, ml — integers where the source is integral

- `distance_m NUMERIC(10,1)` — Garmin reports metres as float; 1dp is plenty (Round1 at the boundary like other numerics). Metres, not km, to avoid a float-precision unit nobody else in the codebase uses; the agent converts for display.
- `avg_power_w INTEGER` — watts are reported and reasoned about as integers (matches `avg_hr`).
- `temperature_c NUMERIC(4,1)` with CHECK `BETWEEN -40 AND 60` — wide enough for any real outdoor session, tight enough to reject unit confusion (°F values like 75 pass, unfortunately, but 98.6-style body-temp °F mistakes are caught; perfect detection is impossible).
- `sweat_loss_ml NUMERIC(10,1)` CHECK `> 0` — ml matches the hydration capability's unit isolation convention.

Per the unit-isolation rule, none of these join any shared Totals struct; they live only on the workout row and its echoes.

### 5. `session_group` filter composes with the existing required window

`GET /workouts` keeps its mandatory `from`/`to` (and the `range_too_large` guard); `session_group` is an additional `AND` predicate, not an alternative lookup path. Rationale: a session's legs are by definition within hours of each other, so a window is never a burden; keeping the window mandatory preserves the existing contract and avoids an unbounded table scan path. The partial index on `session_group` keeps the combined predicate cheap.

### 6. Fueling block echoes `sweat_loss_ml` + `temperature_c` only

`GET /workouts/{id}/fueling` exists to answer "did the fueling work?" — sweat loss and heat are inputs to that judgment (fluid/sodium adequacy); distance, power, and group key are not. Echoing all five would bloat the rehearsal-evaluation payload with performance data the capability explicitly excludes. Matches the rpe/gi precedent of echoing only judgment-relevant fields.

### 7. Bulk path reuses the single-item decode unchanged

`/workouts/bulk` already decodes each item as a `createRequest`; the new fields ride along with per-item validation errors surfacing in the per-item results, exactly like existing fields. No bulk-specific handling.

## Risks / Trade-offs

- **[Free-text `session_group` has no referential integrity]** → Acceptable: single-user system, writer-controlled keys, and the partial index makes orphan detection trivial (`GROUP BY session_group HAVING count(*) = 1`). A future race entity can formalize it.
- **[Garmin's sweat-loss estimate is a model output, not a measurement]** → Stored verbatim; the deferred sweat-rate capability (priorities 6C) treats pre/post weigh-ins as the calibration source and this field as the per-session default. Column name deliberately does not claim "measured".
- **[Mixed PATCH semantics across old vs new fields]** (kcal set-only vs new tri-state) → Documented in swag descriptions; retrofitting old fields is a separate, breaking-ish decision left out deliberately.
- **[°F/°C confusion from non-Garmin writers]** → CHECK range catches gross errors; unit is in the column name, swag docs, and MCP schema description.
- **[MCP integration test expected-tools list]** → No new tools, so the list is unchanged; only arg schemas grow. Risk of forgetting `task swag` → mitigated by the tasks checklist (it is its own task).

## Migration Plan

1. `task migrate:new NAME=widen_workout_ingestion` — **verify `019` is still the next free slot at commit time** (CLAUDE.md notes out-of-band slot-taking has happened).
2. Up: five `ALTER TABLE workouts ADD COLUMN …` statements + `CREATE INDEX workouts_session_group_idx ON workouts (session_group) WHERE session_group IS NOT NULL`.
3. Down: drop the index and the five columns.
4. No back-fill; existing rows read as NULL ("not measured"), matching the rpe/gi precedent.
5. Rollback is the down migration — no data transformation in either direction, so it is lossless for pre-existing data (new-field values are lost on down, by definition acceptable).

## Open Questions

- None blocking. (Whether `recommend_workout_fuel` should eventually prefer `sweat_loss_ml` over its static fluid table is the follow-up change's question, not this one's.)
