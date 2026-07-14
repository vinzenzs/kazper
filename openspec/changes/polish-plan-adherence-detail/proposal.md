## Why

The two open questions `extend-plan-adherence-detail` (archived 2026-07-08) deferred: the missed-sessions list is hard-capped at 50 (a real YTD read can clip silently beyond the flag), and the weekly trend skips empty weeks (a charting consumer needs a continuous axis). Both are request-shaping knobs, not new views.

## What Changes

- `GET /api/v1/workouts/adherence` gains `missed_limit=` (bounds [1, 200], default 50 — existing behavior unchanged when omitted; `missed_sessions_truncated` semantics preserved) and `zero_fill=true` (opt-in: the weekly trend emits every week in span — calendar weeks, or plan-week ordinals in plan mode — with zeroed counts for empty ones).
- The `workout_adherence` MCP tool forwards both optional args.
- No dashboard change (no adherence panel exists yet; the zero-fill exists to enable one later).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `workouts`: 2 ADDED requirements — the tunable missed-list cap and the opt-in zero-filled trend.

## Impact

- **Code:** `internal/workouts` param plumbing into `computeAdherence`'s bucketing; MCP input schema touch; `task swag`.
- **Out of scope:** raising the default, an adherence dashboard panel, per-sport trend splits.
