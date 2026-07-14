## Why

The carried "multisport Phase 4 nicety": Phase 3 gave the multisport template a *template-level* derived `estimated_duration_sec`, but the per-segment breakdown ("swim ~35 min, bike ~2:10, run ~55") is still invisible — the client has to re-derive it from step durations. Read-side polish over existing data.

## What Changes

- Multisport template read responses (single and list) carry a derived `estimated_duration_sec` on **each sport segment**: the sum of its time-bound step durations, null when the segment is not fully time-bounded — the same derivation rule as the template-level value, which remains the sum over fully-bounded segments. Transitions keep their existing explicit durations.
- No migration, no new endpoints/tools (rides the existing template shapes verbatim over REST and MCP).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `multisport-workouts`: 1 ADDED requirement — per-segment estimated durations on template reads.

## Impact

- **Code:** derivation at the multisport template serialization boundary; `task swag`.
- **Out of scope:** distance-based duration estimation (pace-dependent), per-segment durations on materialized planned workouts (template view only, as scoped in the backlog note).
