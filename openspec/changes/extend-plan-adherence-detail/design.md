# Design — extend plan-adherence with missed list + weekly trend

## Context

`plan-adherence-analytics` shipped a pure per-window aggregate over
`AdherenceCandidates` (a minimal per-workout projection) classified once by
`computeAdherence(rows, now)`. This change adds two read-side views over the same
candidate set. No migration; the linkage (`plan_slot_id → plan_slots →
plan_weeks`, `training_plans.start_date` = "the Monday of week 1") already exists.

## Decisions

### 1. Grow the existing endpoint, arrays always-on

Both fields land on `GET /workouts/adherence` as always-present JSON, no query
flags. The only reason to gate was keeping a future `/context/training` embed
lean — but that embed is a **separate** endpoint building its own projection, so
it is unaffected. Always-on keeps the `workout_adherence` MCP tool a single
param-free forward.

### 2. `missed_sessions` — compact, capped, explicitly truncated

Entry shape: `{ id, date, sport, planned_duration_min, planned_tss }`. This needs
only **`w.id`** added to the projection (date/sport/duration/tss are already
loaded). No title/description/`training_focus` — that would widen the projection
and the query for marginal value; "compact" is the deliberate line.

Cap at **50** entries. Plan-scoped a window is naturally bounded, but an unscoped
YTD window (the summary allows a 400-day span) could list many misses. The
continuity notes warn that *silent truncation reads as "covered everything"* — so
when the list is cut we set **`missed_sessions_truncated: true`**. The array is
ordered by date ascending (oldest miss first); truncation drops the tail.

Only `missed` sessions are listed — not upcoming, not unplanned. Naming what was
blown is the coach's question; the counts already cover the other buckets.

### 3. `weekly` trend — plan-week-aware bucketing

Two honest modes, selected by whether `plan_id` is supplied:

```
  plan_id PRESENT → bucket by plan_weeks.ordinal (INNER JOIN already present)
                    week_start = training_plans.start_date + (ordinal-1)*7
                    bucket carries ordinal + phase name (from pw.phase_id → training_phases)
                    off-plan rows already excluded by the join

  plan_id ABSENT  → bucket by calendar week, Monday-start (endurance convention),
                    computed in Go from started_at in the resolved tz
                    ordinal/phase are null
```

Bucketing by `ordinal` (not by derived date) in plan mode is exact and needs no
date arithmetic for the grouping key — `week_start` is derived only for display.
This requires adding **`pw.ordinal`** and **`pw.phase_id`** to the plan-mode
SELECT (plus a join to `training_phases` for the phase name, or a second small
lookup — a LEFT JOIN keeps it one query).

Each bucket: `{ week_start, ordinal?, phase?, completed, missed, adherence_rate,
planned_duration_min, completed_duration_min }`. Per-week `adherence_rate` uses
the same `completed/(completed+missed)`, null when nothing was due that week.
Deliberately no per-week `tss` or by_sport — keep the trend lean.

Buckets are emitted only for weeks that have at least one candidate row. We do
**not** zero-fill empty plan weeks or empty calendar weeks — unlike the
`workout-stats` daily summary (which zero-fills for a continuous heatmap), a
trend of adherence over sessions is meaningful only where sessions exist, and a
zero-filled empty week would show a misleading `adherence_rate: null` row. (Open
to revisiting if the frontend wants a continuous axis.)

### 4. Refactor `computeAdherence` to emit per-bucket alongside the total

`computeAdherence` already classifies each row exactly once. Extend it to fold
each classified row into **both** the running total (unchanged output) **and** a
per-bucket accumulator keyed by the bucket key, so the top-line and the trend are
computed from the same pass — no double classification, no risk of the trend and
the total disagreeing. The bucket key is the plan-week ordinal (plan mode) or the
Monday-of-week date (calendar mode); the row projection carries what's needed to
derive it.

`missed_sessions` is collected in the same pass (append on the missed branch),
sorted by date, then capped at the boundary.

## Risks / edge cases

- **A plan week with only future (upcoming) sessions** → that bucket's
  `adherence_rate` is null (nothing due yet), same null semantics as the top line.
  Intended, not a bug.
- **Unscoped window spanning multiple plans** → calendar-week buckets mix sessions
  from different plans; that's correct for the unscoped "how am I doing lately"
  question. Plan-week alignment is opt-in via `plan_id`.
- **`training_plans.start_date` drift** → `week_start` is derived, display-only;
  the grouping key is `ordinal`, so a wrong start_date mislabels but never
  misgroups.
- **Cap interaction with truncation flag** — the flag is set strictly when the
  pre-cap missed count exceeds the cap, so `len(missed_sessions) == cap &&
  !truncated` is a valid state (exactly 50 misses).

## Open questions

- Cap value (50) is a guess; revisit if a real YTD read feels clipped.
- Zero-filling the trend for a continuous frontend axis — deferred; add only if
  the `/stats` chart needs it.
