## Why

`multisport-phase-2` (archived 2026-06-16) wired multisport templates into the plan and left two explicit open questions about how a brick *reads* once it's in the system. Both are coach-facing polish, not new mechanics: (1) a multisport template has **no duration** anywhere in its API response — a single-sport `workout_templates` row carries `estimated_duration_sec`, but a `multisport_templates` row exposes nothing, so the coach/UI can't see how long a triathlon session takes without materializing it onto a plan; and (2) the training-context **load-by-sport** summary buckets a `multisport` workout under one opaque `multisport` key, so a swim→bike→run brick credits none of its three legs in `by_sport`. This phase closes both, read-time only.

## What Changes

- **Derived duration on a multisport template**: the `multisport-templates` GET/list response gains a computed `estimated_duration_sec` = the sum of every **sport** segment's time-bounded step durations plus every **transition** segment's time duration. It SHALL be omitted (null) when any segment is not fully time-bounded (a distance/open/lap-button step, or a non-time transition) — mirroring how the single-sport materializer already treats a non-time program. Computed at the response boundary; no column, no migration.
- **Per-segment-sport decomposition in `by_sport`**: the `GET /context/training` recent-load summary SHALL count a `multisport` workout once **per non-transition segment sport** (a brick adds one each to swim/bike/run) instead of one `multisport` bucket, so the coach sees real per-discipline volume. The workout row's own `sport` stays `multisport`; only the `by_sport` aggregation decomposes it, resolving the segment sports from the workout's `multisport_template_id`. When the template can't be resolved (repo unset / template gone), it falls back to the single `multisport` bucket and never errors. `count`, `total_duration_min`, and `total_kcal` are unchanged (the workout is still one session).
- No new MCP tools; no garmin-bridge, materialize, or push-path changes.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `multisport-workouts`: a multisport template's read response carries a derived `estimated_duration_sec` (sum of segment durations; null when not fully time-bounded).
- `coach-context`: the training-context recent-load `by_sport` summary decomposes a `multisport` workout into its constituent segment sports, falling back to the `multisport` bucket when the template is unresolvable.

## Impact

- **Code**: `internal/multisport/` (derive `estimated_duration_sec` at the response boundary, reusing `workouttemplates.SumTimedDurationSec`); `internal/coachcontext/` (cross-inject the multisport repo, decompose `multisport` rows in `summarize`'s `by_sport`); `internal/httpserver/server.go` wiring.
- **Docs**: `task swag` for the multisport-template response shape.
- **No migration** (both are computed on read), no new tools, no bridge change. Builds on `multisport-phase-2`.
