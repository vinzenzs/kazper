# Extend plan-adherence with a missed-session list and a weekly trend

## Why

`GET /workouts/adherence` (from `plan-adherence-analytics`) answers *how well was
the plan followed over this window* with counts, a rate, planned-vs-actual volume,
and a `by_sport` split. Two questions it can't answer were deferred at the time,
both flagged in that change's design as clean follow-ups:

- **Which sessions were missed?** The coach gets a `missed` count but can't *name*
  the sessions to talk about them ("you skipped Tuesday's 60-min run").
- **Is adherence trending?** A single window total can't show whether the athlete
  is slipping or recovering across the weeks in the window.

The linkage needed for both already exists — every planned/completed-from-plan
workout carries `plan_slot_id → plan_slots → plan_weeks` — so this is a read-side
enrichment with no schema change.

## What Changes

Grow the **same** `GET /workouts/adherence` response with two new always-present
fields (no new query params, no new endpoint):

- **`missed_sessions`** — a compact list of the overdue-and-unfulfilled sessions
  (`id`, `date`, `sport`, `planned_duration_min`, `planned_tss`). Capped, with an
  explicit **`missed_sessions_truncated`** boolean so a truncated list never reads
  as "these were all of them".
- **`weekly`** — a per-week trend array. Each bucket carries `week_start`, the
  four counts collapsed to `completed`/`missed`, `adherence_rate`, and
  `planned_duration_min`/`completed_duration_min`. Bucketing is **plan-week-aware**:
  when `plan_id` is supplied, buckets align to the plan's weeks (`ordinal` + phase
  name, week_start derived from `training_plans.start_date`); with no `plan_id`,
  buckets are calendar weeks (Monday-start).

The existing `workout_adherence` MCP tool forwards the richer body verbatim — no
new tool, no schema/golden change.

## Impact

- **Specs:** `workouts` — two ADDED requirements (missed list, weekly trend). The
  existing adherence requirement is unchanged.
- **Code:** `internal/workouts/` — widen the `AdherenceCandidates` projection
  (`w.id`; `pw.ordinal`/`pw.phase_id` in plan mode), refactor `computeAdherence`
  to emit per-bucket aggregates alongside the total, add the two response fields
  and a cap constant. No new package, **no migration**.
- **MCP:** none beyond the wider body (verbatim forward).
- **Docs:** `task swag` regenerates `docs/` for the grown response shape.

## Non-goals

- Rich `missed_sessions` entries (title/description/`training_focus`) — kept
  **compact** deliberately; the projection stays minimal.
- Per-week volume beyond duration (no per-week `tss`, no by_sport-per-week).
- Embedding adherence into `/context/training` — still a separate follow-up with
  its own lean projection.
- Query-flag gating of the arrays — they are always present; the leanness concern
  belongs to the future context embed, not this endpoint.
