## Why

The modern long-course metric both gap analyses flagged: not what power the athlete can produce, but what power survives deep into a ride. Kazper's best-efforts ladder is fatigue-blind — a fresh 20-minute best and one set 2000 kJ into a ride count the same — yet fatigue resistance is exactly what decides an Ironman bike split or a late-race attack. The inputs exist (stored 1 Hz power streams, the ingest/recompute pipeline); what's missing is tiering best efforts by the work done before them.

## What Changes

- **Migration (next free slot, currently `061`):** `workout_best_efforts` gains `kj_tier SMALLINT NOT NULL DEFAULT 0` and the unique key widens to (workout, metric, duration, kj_tier) — tier 0 is exactly today's rows, so existing data and every existing query keep their semantics.
- Stream ingest (and the existing recompute path) additionally computes, for power streams, the mean-maximal 1m/5m/20m **after** each accumulated-work tier (500/1000/1500/2000 kJ) — the best effort whose window starts after that much work — storing rows only for tiers the ride reaches. Historical rides backfill via the existing `recompute` endpoint.
- `GET /api/v1/workouts/durability?from=&to=&tz=` — per duration: the fresh (tier-0) windowed best vs each tier's windowed best with `fade_pct`, each with contributing workout/date; tiers with no data in the window are omitted.
- New `durability` MCP tool (read tier, one GET, verbatim).
- `/stats` gains a durability panel: fade per tier per duration, empty state until tiered data exists.
- Lands in `internal/effortanalytics/` (owns the ladder and its ingest derivation).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `effort-analytics`: 1 MODIFIED requirement (the best-efforts ingest requirement gains kJ-tiering) + 2 ADDED (the durability endpoint, the MCP tool).
- `coach-dashboard`: 1 ADDED requirement — the stats durability panel.

## Impact

- **Code:** migration on `workout_best_efforts`; `internal/effortanalytics/` ingest derivation + windowed tiered-MAX query + handler; `internal/activitystreams` recompute delegation unchanged in shape (it already calls `ComputeAndReplace`); `apps/web` panel.
- **API/MCP:** one GET, one read tool, golden regen additive, `task swag`.
- **Operational:** historical durability appears only after re-running recompute over old workouts (or natural re-syncs) — same posture as the original best-efforts backfill.
- **Out of scope (deferred):** speed/pace durability for run, heart-rate drift as a durability proxy, per-workout durability detail endpoints, configurable tiers/durations (constants v1).
