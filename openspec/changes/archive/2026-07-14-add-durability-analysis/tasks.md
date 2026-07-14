# Tasks

## 1. Schema

- [x] 1.1 Verify the on-disk migration head (currently `060`; two sibling proposals also carry migrations — take the next free slot), then `task migrate:new NAME=add_best_effort_kj_tiers`: `ADD COLUMN kj_tier SMALLINT NOT NULL DEFAULT 0` + widen the unique constraint to (workout_id, metric, duration, kj_tier); down: delete tier > 0 rows, restore constraint, drop column
- [x] 1.2 Audit every existing `workout_best_efforts` consumer (power-curve, cp-model, power-profile queries + test helpers) to pin `kj_tier = 0`; add a regression test asserting tiered rows never leak into the fresh curve

## 2. Tier derivation (ingest + recompute)

- [x] 2.1 Extend the mean-maximal computation in `internal/effortanalytics/` with cumulative-kJ tiering (window start at/after tier; 1m/5m/20m × 500/1000/1500/2000 kJ; power only; rows only for reached tiers) inside `ComputeAndReplace`
- [x] 2.2 Unit tests: synthetic ride reaching 1600 kJ (tiers 500/1000/1500 written, 2000 absent; hand-computed tier bests), short ride (no tiers), window-start boundary exactness, recompute reproduces ingest
- [x] 2.3 Integration: re-post replaces tiered rows; delete-cascade still clean

## 3. Endpoint

- [x] 3.1 Windowed tiered-MAX query (per duration × tier, contributing workout/date) + `GET /workouts/durability` handler with `fade_pct`, tier omission, `no_tiered_data` reason, power-curve range/tz contract
- [x] 3.2 Integration tests: fade happy path, tier omission, no-tiered-data reason, empty window, range 400 matrix, read-only
- [x] 3.3 `task swag`

## 4. MCP

- [x] 4.1 `durability` read tool (one GET, verbatim; description notes the recompute backfill)
- [x] 4.2 Golden regen (additive) + registry/integration green

## 5. Dashboard

- [x] 5.1 `useDurability` hook + types; `/stats` fade grid with window selector + explanatory empty state
- [x] 5.2 Web tests (grid, empty state) — `tsc` + vitest green

## 6. Verification & backfill

- [x] 6.1 `task vet` + full suite green (`-p 1` rerun on boot-contention flakes); direct-insert test helpers updated for the widened key
- [ ] 6.2 Live: recompute a handful of long historical rides, confirm tiers appear and the fade read passes the eyeball test
