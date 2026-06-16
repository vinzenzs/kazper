## Context

The planned↔completed linkage is fully in place: `materialize` stamps `plan_slot_id` on each planned `workouts` row (`status='planned'`), and reconciliation (forward at ingest, reverse at materialize) flips a matched row to `status='completed'` while retaining `plan_slot_id`/`template_id`. So every workout row already carries everything adherence needs — `status`, `plan_slot_id`, the planned/actual time window (`started_at`/`ended_at`), and `tss` — but the only reads over it are the row lists (`GET /workouts`, `GET /context/training`). The workouts service has `s.loc` and uses `time.Now()` directly; there is a `ListWindow(from, to, sessionGroup, status)` repo read and a `plan_slots → plan_weeks → training_plans` chain for plan scoping.

## Goals / Non-Goals

**Goals:**
- One read that answers "did the athlete follow the plan over this window?" — completion rate, missed/completed/upcoming/unplanned counts, planned-vs-actual volume, by sport.
- Reuse the existing columns and timezone handling; no new state, no migration.
- Coach-groundable: mirrored as an MCP tool.

**Non-Goals:**
- Per-week trend series or charts (a window total is enough; trends are a later read).
- Recovery- or readiness-adjusted adherence, or auto-rescheduling missed sessions.
- Any write — adherence never mutates a workout's status (that's `fulfill`/`unfulfill`).
- Embedding adherence into the `GET /context/training` bundle (a clean follow-up once the shape settles).

## Decisions

### D1: A window read on `workouts`, optionally plan-scoped
`GET /workouts/adherence?from=&to=&tz=&plan_id=` lives in the `workouts` capability (it owns the rows). `from`/`to` are local dates (resolved in `tz`, defaulting to the configured zone); `plan_id` is optional. Chosen over a `training-plan` endpoint (the data and query layer are in `workouts`; a window read is more flexible than plan-only) and over extending `coach-context` (keep it an independently-callable read; coach-context can embed it later).

### D2: Execution-state classification
Each in-window workout is bucketed once, from `status` + `plan_slot_id` + `started_at` vs **now** (in `tz`):
- **completed** — `status='completed'` AND `plan_slot_id` present (a planned session that was done; reconciliation guarantees the fulfilled row keeps its slot).
- **missed** — `status='planned'` AND `started_at` < now (overdue, never fulfilled).
- **upcoming** — `status='planned'` AND `started_at` ≥ now (not yet due).
- **unplanned** — `status='completed'` AND `plan_slot_id` IS NULL (extra, off-plan work).
A completed row without a slot is never "missed"; an upcoming planned row never counts against adherence.

### D3: Adherence rate over due sessions only
`adherence_rate = completed / (completed + missed)`, rounded to 1dp at the boundary, **null** when `completed + missed == 0` (nothing was due in the window). Upcoming and unplanned are reported but excluded from the rate, so a future-heavy window doesn't dilute it and extra work doesn't inflate it.

### D4: Pull rows, aggregate in a pure function
The repo returns the windowed candidate rows (optionally plan-joined); the service classifies and aggregates in a pure `computeAdherence(rows, now) AdherenceSummary`. Keeps the coaching logic unit-testable without a DB and the SQL trivial. A window is a couple of weeks of rows — cheap. (A SQL `GROUP BY` aggregate was rejected: it would push the now-comparison and by-sport/duration math into SQL, harder to test and to evolve.)

### D5: Plan scoping via join, no Go dependency
When `plan_id` is set, the repo query joins `workouts.plan_slot_id → plan_slots.id → plan_weeks.plan_id = $plan`. This is pure SQL inside the workouts repo — no `trainingplan` package import, no cycle. An unplanned (no-slot) completed row is excluded when `plan_id` is set (it can't belong to a plan).

### D6: Planned-vs-actual volume
`planned_duration_min` sums the planned window (`ended_at − started_at`) of **completed + missed** sessions (what the plan asked for); `completed_duration_min` sums the actual window of **completed** sessions. Same split for `tss` (`planned_tss` over completed+missed where present, `completed_tss` over completed). All rounded at the boundary; a sum over zero present values serializes as null. `by_sport` maps each sport to its completed/missed counts.

### D7: MCP mirror + golden
Add a `workout_adherence` tool in `registry_workouts.go` (one HTTP GET, args `from`/`to`/`tz`/`plan_id`). Bump the `mcp_integration_test.go` expected-tools list by one and regenerate the announced-schema golden (a new tool — the documented `goldengen` step, as with `swim_pace`/`cadence`).

## Risks / Trade-offs

- **"Missed" depends on the wall clock** → a planned session earlier *today* is already "missed" the moment its start passes. Acceptable and correct for a same-day read; the coach reads adherence for past windows where it's unambiguous. Documented in the requirement.
- **Multisport planned rows** count under sport `multisport` in `by_sport` and in the totals like any planned session — correct (a brick is one planned session); no special-casing.
- **A window with only future sessions** yields `adherence_rate: null` (nothing due) rather than 0 — intended, so an early-week read isn't misread as total failure.

## Migration Plan

Pure read, no DB migration. Deploy ships the endpoint + tool; `task swag` regenerates docs, `goldengen` regenerates the MCP schema baseline. Rollback = revert; nothing persisted.

## Open Questions

- Should the response also return the **list** of missed sessions (ids/dates/sports), not just the count, so the coach can name them? Lean yes but keep it small — include a compact `missed_sessions` array (id, date, sport). Decide during apply; the count is the minimum.
- Per-week breakdown (a trend array) — deferred to a follow-up once the single-window shape is in use.
