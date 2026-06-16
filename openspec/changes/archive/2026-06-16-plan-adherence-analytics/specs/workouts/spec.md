## ADDED Requirements

### Requirement: A read computes plan-adherence analytics over a window

The system SHALL expose `GET /workouts/adherence` (authenticated) accepting
`from`, `to`, `tz`, and an optional `plan_id`, returning plan-adherence analytics
over the `[from, to]` local-date window. Each workout in the window SHALL be
classified once, from `status`, `plan_slot_id`, and `started_at` compared to the
current time in `tz`:

- **completed** — `status='completed'` with a `plan_slot_id` (a planned session
  that was done),
- **missed** — `status='planned'` with `started_at` before now (overdue),
- **upcoming** — `status='planned'` with `started_at` at or after now,
- **unplanned** — `status='completed'` with no `plan_slot_id` (off-plan work).

The response SHALL report the four counts, an **`adherence_rate`** equal to
`completed / (completed + missed)` rounded at the boundary and **null** when no
sessions are due (`completed + missed == 0`), a planned-vs-actual volume
(`planned_duration_min` over completed + missed sessions vs `completed_duration_min`
over completed sessions, and the same split for `tss` where present), and a
`by_sport` breakdown of completed/missed counts. When `plan_id` is supplied the
window SHALL be restricted to workouts whose `plan_slot_id` belongs to that plan
(joined through `plan_slots`/`plan_weeks`), which excludes unplanned rows. The
read SHALL NOT mutate any workout. Numeric fields SHALL be rounded at the response
boundary and a sum over zero present values SHALL serialize as null.

#### Scenario: Completed and missed sessions drive the adherence rate

- **WHEN** a window contains three planned sessions whose dates have passed —
  two fulfilled (`status='completed'`, `plan_slot_id` set) and one still
  `planned` — and `GET /workouts/adherence` is called
- **THEN** `completed` is 2, `missed` is 1, and `adherence_rate` is `0.7` (2 / 3)

#### Scenario: Upcoming sessions are excluded from the rate

- **WHEN** the window also contains a `planned` session dated in the future
- **THEN** it is counted under `upcoming` and does NOT change `completed`,
  `missed`, or `adherence_rate`

#### Scenario: Off-plan completed work is unplanned, not counted against adherence

- **WHEN** a completed workout with no `plan_slot_id` falls in the window
- **THEN** it is counted under `unplanned` and excluded from `adherence_rate`

#### Scenario: Adherence rate is null when nothing is due

- **WHEN** the window contains only upcoming planned sessions (none overdue, none
  completed)
- **THEN** `adherence_rate` is null and `completed`/`missed` are 0

#### Scenario: Planned-vs-actual volume and by-sport are reported

- **WHEN** the window has completed and missed sessions across sports
- **THEN** the response reports `planned_duration_min` (completed + missed) and
  `completed_duration_min` (completed only), and a `by_sport` map of
  completed/missed counts per sport

#### Scenario: plan_id scopes the window to one plan

- **WHEN** `GET /workouts/adherence` is called with a `plan_id`
- **THEN** only workouts whose `plan_slot_id` belongs to that plan are considered
- **AND** off-plan (no-slot) completed workouts are excluded

#### Scenario: The adherence read is exposed as an MCP tool

- **WHEN** the agent calls the adherence MCP tool
- **THEN** the MCP server issues exactly one `GET /workouts/adherence` request and
  forwards the response verbatim
