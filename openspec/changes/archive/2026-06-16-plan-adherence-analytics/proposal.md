## Why

The system now links planned and completed work tightly — materialize stamps a `plan_slot_id` on each planned workout, and reconciliation (forward + reverse) fulfills the planned row in place when the activity arrives — but nothing **reads back** how well the athlete actually followed the plan. The coach can list recent completed workouts and upcoming planned ones (via `GET /context/training`), yet has to eyeball "did I hit my sessions?" There's no completion rate, no count of missed sessions, no planned-vs-actual volume. For an 18-week build toward a race, adherence is a first-order coaching signal — a string of missed long rides matters more than any single day's macros. The data to compute it already exists on the `workouts` table (`status`, `plan_slot_id`, the planned window vs the actual window); this change surfaces it as a read.

## What Changes

- Add a read **`GET /workouts/adherence`** (authenticated) accepting `from`, `to`, `tz`, and an optional `plan_id`, returning plan-adherence analytics over the window:
  - **Counts** by execution state: `completed` (a planned session that was done — `status='completed'` with a `plan_slot_id`), `missed` (a planned session now overdue — `status='planned'` with `started_at` before now), `upcoming` (`status='planned'`, `started_at` at/after now), and `unplanned` (a completed session with no `plan_slot_id` — extra work).
  - **`adherence_rate`** = `completed / (completed + missed)` over **due** sessions only (upcoming excluded), rounded; null when no sessions are due.
  - **Planned-vs-actual volume**: `planned_duration_min` (the planned windows of completed + missed sessions) vs `completed_duration_min` (the actual durations of completed sessions), and the same for `tss` when present.
  - **`by_sport`** breakdown of completed / missed counts.
  - When `plan_id` is supplied, the window is restricted to workouts whose `plan_slot_id` belongs to that plan (join `plan_slots → plan_weeks`).
- Mirror it 1:1 as an MCP tool (`workout_adherence`) so the coach agent can ground on it; the MCP integration expected-tools list gains one entry.
- Read-only and additive: no writes, no lifecycle change, no migration. "Now" comes from the server clock in the resolved timezone; numbers are rounded at the response boundary.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workouts`: a new `GET /workouts/adherence` read computes plan-adherence analytics (completed/missed/upcoming/unplanned counts, adherence rate, planned-vs-actual duration & TSS, by-sport) over a date window, optionally scoped to a plan, mirrored as an MCP tool.

## Impact

- **Code**: `internal/workouts/` (a window aggregation query with optional plan join + the analytics computation in the service; handler + swag annotations), `internal/agenttools/registry_workouts.go` (new `workout_adherence` tool), `internal/mcpserver/mcp_integration_test.go` expected-tools list (+1), `internal/httpserver/server.go` route registration is already covered by the workouts handler group.
- **Docs**: `task swag` for the new endpoint + response shape; regenerate the MCP announced-schema golden for the new tool.
- **No migration** — pure read over existing columns (`status`, `plan_slot_id`, `started_at`/`ended_at`, `tss`).
- **Coupling**: builds on the planned↔completed linkage from `add-training-plan`, `add-workout-reconciliation`, and `reverse-direction-workout-reconciliation`. Orthogonal to coach-context (which could later embed this summary, a follow-up).
