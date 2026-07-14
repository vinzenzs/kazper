# Tasks

## 1. Target simulation (pure, unit-tested)

- [x] 1.1 `internal/pmc/target.go`: per-day target TSS from phases (weekly/7, 0 + `target_declared:false` for gaps/null targets), EWMA reuse seeded from `seed_ctl`, deltas, summary (current delta, 14-day trend difference, both end projections)
- [x] 1.2 Unit tests: hand-computed short-macrocycle fixture (flat target → known CTL curve), gap decay + flags, seeded-start correctness, projections (`_planned` vs `_current` diverge as expected after under-training), all-null-targets → nil trajectory, boundary rounding only

## 2. Endpoint

- [x] 2.1 Narrow macrocycle/phases read interface (resolve active by containing-today + latest-start tie-break; list linked phases with targets) wired in `httpserver.Run()`
- [x] 2.2 `GET /performance/pmc/target-trajectory` handler: optional `macrocycle_id`, `macrocycle_not_found`, `targets_missing` degradation, actual-CTL series + missing-TSS surfacing via the existing PMC service, PMC `tz` vocabulary
- [x] 2.3 Integration tests: active-default happy path, explicit id, gap-flagged spans, targets-missing, unknown-id/no-active 404s, future-days-target-only, tz handling, nothing persisted
- [x] 2.4 `task swag`

## 3. MCP

- [x] 3.1 `pmc_target_trajectory` read tool (optional `macrocycle_id`/`tz`; description: vs `pmc_series`, active default)
- [x] 3.2 Golden regen (additive) + registry/integration green

## 4. Dashboard

- [x] 4.1 `useTargetTrajectory` hook + types; dashed target overlay on the PMC panel (muted undeclared spans) + on-plan readout; absent on 404/`targets_missing`/error
- [x] 4.2 Web tests (overlay renders, absent states, readout values) — `tsc` + vitest green

## 5. Verification

- [x] 5.1 `task vet` + full suite green (`-p 1` rerun on boot-contention flakes)
- [ ] 5.2 Live sanity: run against the current macrocycle; check `seed_ctl` matches the PMC series at start date and `current_delta` squares with the eyeball read
