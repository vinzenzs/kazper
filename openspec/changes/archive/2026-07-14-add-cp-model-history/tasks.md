# Tasks

## 1. Anchor loop

- [x] 1.1 `internal/effortanalytics/`: Monday-anchor enumeration + per-anchor reuse of the existing fit; `window_days` validation; unit tests (anchor alignment, trailing-window bounds, null-anchor retention, `window_days` matrix)
- [x] 1.2 `GET /workouts/cp-model/history` handler (power-curve range/tz contract, 400-day cap) + integration tests (season series, gapped anchors, param 400s, read-only) + `task swag`

## 2. MCP

- [x] 2.1 `cp_model_history` read tool + golden regen (additive) + registry/integration green

## 3. Dashboard

- [x] 3.1 `useCPModelHistory` hook + trend line with gaps + configured-FTP step overlay (athlete-config history fetch); web tests (gap rendering, overlay, absent states) — `tsc` + vitest green

## 4. Verification

- [x] 4.1 `task vet` + full suite green (`-p 1` on boot-contention flakes); live: season read, eyeball CP trend vs known FTP bumps
