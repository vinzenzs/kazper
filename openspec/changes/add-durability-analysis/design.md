## Context

`effort-analytics` derives mean-maximal best efforts at ingest (`ComputeAndReplace`, called from activity-streams' `POST /workouts/{id}/streams` and its recompute path) into `workout_best_efforts`, keyed (workout, metric, duration); the power-curve/CP/profile endpoints aggregate windowed MAX over it. Durability ("fatigue resistance", WKO4/Intervals.icu lineage) asks the same mean-maximal question restricted to efforts starting after N kJ of accumulated work — computable from the stored 1 Hz power stream in one pass.

The alternative — computing tiers on read by scanning every stored stream in the window — would turn a 180-day durability read into hundreds of full-stream scans per request. The ingest/recompute pipeline exists precisely to pay per-workout costs once.

## Goals / Non-Goals

**Goals:**
- Persist per-workout kJ-tiered best efforts exactly like the fresh ladder: derived at ingest, reproducible via recompute, replaced on re-post.
- A windowed read answering "how much does my 5-minute power fade after 1500 kJ" with contributing rides visible.
- Zero disturbance to existing best-efforts semantics or consumers.

**Non-Goals:**
- Run/swim durability (speed fade mixes terrain; HR drift is a different metric).
- Per-workout durability views, configurable tiers, or a durability *score* (a single scalar would hide which duration/tier fades — the table is the product).
- Eager backfill tooling beyond the existing recompute endpoint.

## Decisions

### D1 — Widen `workout_best_efforts` with `kj_tier` (default 0) rather than a new table
Tier 0 *is* the existing ladder — same metric, same durations, tier merely qualifies "after how much work" (fresh = after 0 kJ). One table keeps ingest one write path and lets the tiered windowed-MAX query be the existing one plus a tier predicate. Existing rows get tier 0 via the column default; the unique key widens to include it. Every existing query gains an implicit/explicit `kj_tier = 0` — enumerated and asserted in tests so no consumer silently starts mixing tiers.

- **Why not `durability_efforts`?** A parallel table duplicates the replace-on-repost machinery, the recompute path, and the projection — for rows shaped identically.

### D2 — Tiers 500/1000/1500/2000 kJ; durations 1m/5m/20m; window starts after the tier
Fixed tiers (constants precedent) spanning ~1–5 h of endurance riding; durations chosen where fatigue shows (sprint fade is neuromuscular noise; 20m-after-2000kJ is the Ironman question). An effort qualifies for tier N when its window's **first sample** lies after cumulative work ≥ N kJ — a strict "produced while already that deep" reading. Rows are written only for tiers the ride actually reaches, so short rides add nothing.

### D3 — Ingest-time derivation, recompute-path backfill
The tier computation runs inside the existing `ComputeAndReplace` (one extra pass over the power array — cumulative kJ is a running sum). Historical rides get tiers when `recompute` re-derives them from stored streams — the exact backfill path `persist-activity-streams` established. No new operational surface; the durability panel's empty state names the recompute route.

### D4 — Read shape: fresh-vs-tier fade table
`GET /workouts/durability?from=&to=&tz=`: per duration, `fresh` (tier-0 windowed best) and per-tier entries `{kj_tier, watts, fade_pct, workout_id, date}` where `fade_pct = (fresh − tier) / fresh × 100` (`Round1`; fresh from the same window so the comparison is internally consistent). Tiers absent in the window are omitted; a window with no tiered rows at all returns the fresh column plus `reason: "no_tiered_data"`. Power-curve range/`tz` error contract; compute-on-read over stored rows only (no stream scans).

### D5 — MCP `durability`, read tier, verbatim
Compact table, full body. Description notes the recompute backfill dependency for historical windows.

### D6 — Dashboard: fade table on `/stats`
Durations × tiers grid with fade coloring, window selector, empty state pointing at recompute. No chart in v1 — the grid is the honest form until there's enough tiered data to trend.

## Risks / Trade-offs

- **Row growth** — ≤ 12 extra rows per long ride (3 durations × ≤ 4 tiers); negligible.
- **Unique-key widening is the risky migration step** — mitigated: default 0 back-fills atomically, the up-migration is `ADD COLUMN` + constraint swap, and the full suite's direct-insert helpers get the new column exercised.
- **Fresh-vs-tier compares different rides** — inherent to windowed durability (the fresh best and the 2000 kJ best rarely share a ride); per-entry workout/date keeps it auditable. A same-ride constraint would need per-workout views (deferred non-goal).
- **kJ ≠ physiological fatigue** (low-intensity kJ fatigue less) — accepted; work-based tiers are the established convention and the only parameter-free one.

## Migration Plan

Migration on the next free slot (currently `061` — verify the on-disk head at apply time; two other proposed changes also carry migrations). Up: add column with default + widen unique constraint; down: delete tier > 0 rows, restore the narrow constraint, drop the column. Rollout: new syncs tier automatically; history via recompute at the operator's pace.

## Open Questions

- Should the endpoint offer a same-ride mode (fresh and tiered from one workout) once per-workout durability views exist? (Deferred with them.)
- Tier ceiling — is 2000 kJ enough for full-distance days, or add 2500? (v1: stop at 2000; adding a tier is additive.)
