# Tasks

## 1. Detection (pure, unit-tested)

- [x] 1.1 `internal/activitystreams/intervals.go`: 30 s centered rolling mean → Otsu threshold over the smoothed histogram → span assembly (≤ 30 s gap-merge, ≥ 60 s minimum) → intervals/rests/summary; bimodality gate → `no_distinct_efforts`; no I/O
- [x] 1.2 Unit tests: synthetic 5×4 min fixture (exact boundaries + threshold), dip-inside-effort merge, sub-60 s burst discard, steady-ride null result, trimodal ride (documented lumping behavior), kJ boundary rounding

## 2. Endpoint

- [x] 2.1 `GET /workouts/{id}/intervals` handler: reuse stored-stream read; sentinels `workout_not_found`/`streams_not_found`/`power_stream_missing`; nothing persisted
- [x] 2.2 Integration tests: structured ride happy path, steady-ride reason, sentinel matrix, no row mutation
- [x] 2.3 `task swag`

## 3. MCP

- [x] 3.1 `detect_intervals` read tool (one GET, verbatim full body)
- [x] 3.2 Golden regen (additive) + registry/integration green

## 4. Dashboard

- [x] 4.1 `useDetectedIntervals` hook + types; detail-page table beside splits, absent on `no_distinct_efforts`/no-power/error
- [x] 4.2 Web tests (table renders, absent states) — `tsc` + vitest green

## 5. Verification

- [x] 5.1 `task vet` + full suite green (`-p 1` rerun on boot-contention flakes)
- [ ] 5.2 Live sanity: run detection over a known real interval session and a steady endurance ride; check boundaries and the null result against the actual sessions
