## Why

**Live-validation finding (coach, 2026-07-16):** the heat read anchors its forecast window at the planned workout's `started_at` verbatim (`internal/heat/service.go`) — but scheduled/materialized planned workouts are midnight-anchored (Garmin's calendar is date-only), so the read scores **pre-dawn hours**. Verified live: the server reported `heat_load_c: 20.4` (the ~05:00 forecast) for a session that, ridden at 10:00, would face ~27.5 °C apparent heat — a 7 °C under-read eight days before a race. The number was right for the wrong hour.

## What Changes

- **Midnight is a sentinel, not a start time**: a planned workout whose `started_at` local time-of-day is exactly 00:00:00 is treated as "start time unknown" — the heat window re-anchors at a configured **`DEFAULT_TRAINING_START`** (local `HH:MM`, default `06:00`), echoed as `assumed_start`. Read-side only, so it fixes every existing midnight-anchored row retroactively with no migration and no write-path change.
- **What-if starts**: the heat read gains an optional `start=HH:MM` parameter overriding the anchor (`400 start_invalid` on malformed) — the "before 07:00 vs past 09:00" comparison the coach just did by hand becomes two calls. The response always echoes the effective window and `start_source: workout | assumed | param`.
- A planned workout carrying a **real (non-midnight) time** in `started_at` anchors there, no assumption — setting the true start via the existing workout PATCH is the durable per-session fix (verified as a task).
- `/context/daily`'s heat block uses the same anchoring semantics (it can't take params; the assumed default applies, `assumed_start` visible).
- The `workout_heat` MCP tool gains the optional `start` arg.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `heat-adjustment`: 1 MODIFIED requirement — window anchoring semantics, the `start` parameter, and the echoes added to the heat-read requirement.

## Impact

- **Code:** `internal/heat/` window derivation + param; `DEFAULT_TRAINING_START` config (validated `HH:MM`); MCP arg (golden input-schema touch); `task swag`.
- **Out of scope:** stamping real times at materialize/schedule (read-side sentinel covers it retroactively; write-side stamping can follow if the assumption annoys), per-plan-slot preferred times, hour-by-hour heat curves (the `start` param covers the practical question).
