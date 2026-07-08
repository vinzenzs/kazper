## ADDED Requirements

### Requirement: The adherence read lists the missed sessions

The `GET /workouts/adherence` response SHALL include a **`missed_sessions`** array
naming the sessions classified as **missed** (`status='planned'` with `started_at`
before the current time in `tz`). Each entry SHALL be compact — `id`, `date` (the
session's local date), `sport`, `planned_duration_min`, and `planned_tss` (null
when the planned session carries no TSS) — and the array SHALL be ordered by date
ascending.

The array SHALL be capped at a fixed maximum. When the number of missed sessions
exceeds the cap the response SHALL drop the tail and set
**`missed_sessions_truncated`** to `true`; otherwise `missed_sessions_truncated`
SHALL be `false`. The list SHALL contain only missed sessions — not upcoming and
not unplanned work. Under `plan_id` scoping the list SHALL be restricted to that
plan's sessions, consistent with the counts.

#### Scenario: Missed sessions are named compactly

- **WHEN** a window contains two overdue `planned` sessions that were never
  fulfilled — a 60-minute run and a 90-minute ride — and `GET /workouts/adherence`
  is called
- **THEN** `missed_sessions` contains two entries, oldest first, each with `id`,
  `date`, `sport`, `planned_duration_min`, and `planned_tss`, and
  `missed_sessions_truncated` is `false`

#### Scenario: Only missed sessions appear in the list

- **WHEN** the window also contains a fulfilled (completed-from-plan) session, an
  upcoming `planned` session, and an off-plan completed workout
- **THEN** none of those three appear in `missed_sessions`

#### Scenario: An oversized list is truncated with an explicit flag

- **WHEN** the number of missed sessions in the window exceeds the cap
- **THEN** `missed_sessions` contains exactly the cap's worth of entries (the
  oldest), and `missed_sessions_truncated` is `true`

### Requirement: The adherence read reports a per-week trend

The `GET /workouts/adherence` response SHALL include a **`weekly`** array giving
per-week adherence over the window. Each bucket SHALL report `week_start` (the
local date of the week's first day), `completed`, `missed`, an `adherence_rate`
equal to `completed / (completed + missed)` rounded at the boundary and **null**
when nothing was due that week, and `planned_duration_min` (over that week's
completed + missed sessions) and `completed_duration_min` (over that week's
completed sessions). A bucket SHALL be emitted only for a week that contains at
least one candidate session; empty weeks SHALL NOT be zero-filled.

Bucketing SHALL be plan-week-aware. When `plan_id` is supplied, sessions SHALL be
grouped by their plan week (`plan_weeks.ordinal`), each bucket additionally
reporting `ordinal` and the week's phase name (null when the week has no phase),
with `week_start` derived from the plan's `start_date`. When `plan_id` is absent,
sessions SHALL be grouped by calendar week starting Monday, and `ordinal` and
phase SHALL be null. The trend SHALL be consistent with the top-level counts —
each classified session contributes to exactly one bucket and to the window total.

#### Scenario: Calendar-week trend without a plan

- **WHEN** `GET /workouts/adherence` is called over a multi-week window with no
  `plan_id`, and sessions fall across two Monday-started calendar weeks
- **THEN** `weekly` has one bucket per week that contains sessions, each with
  `week_start`, counts, `adherence_rate`, `planned_duration_min`, and
  `completed_duration_min`, and `ordinal`/`phase` are null

#### Scenario: Plan-week trend aligns to the plan's weeks

- **WHEN** `GET /workouts/adherence` is called with a `plan_id` spanning two plan
  weeks
- **THEN** each `weekly` bucket reports the plan week's `ordinal`, its phase name
  (or null), and a `week_start` derived from the plan's `start_date`, and off-plan
  work is excluded

#### Scenario: A week with only future sessions has a null rate

- **WHEN** a week in the window contains only `planned` sessions dated in the
  future
- **THEN** that bucket's `adherence_rate` is null and its `missed` is 0
