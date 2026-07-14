# Tasks

## 1. Fit math (pure, unit-tested)

- [x] 1.1 `internal/effortanalytics/cpmodel.go`: OLS fit in work–time form over (duration, watts) points → `cp_watts`, `w_prime_j`, `r_squared`, power-space `rmse_w`; no I/O
- [x] 1.2 Validity-band selection ([120s, 1800s]) + gates: <3 in-band durations → `insufficient_points`; longest < 3× shortest → `span_too_narrow`
- [x] 1.3 Unit tests: hand-computed fixture (exact CP/W′ from synthetic points on a known line), noisy fixture (R²/RMSE), every gate branch, band boundary durations (120s in, 60s/3600s out), rounding at the boundary only

## 2. Endpoint

- [x] 2.1 Reuse/widen the windowed per-duration MAX projection (power metric, completed workouts, contributing workout id + date) for the fit's input points
- [x] 2.2 `GET /workouts/cp-model?from=&to=&tz=` handler: power-curve range/tz contract (shared error vocabulary, ≥1-year support), `200` null-model degradation with `reason` + found points, no persistence, no athlete-config read
- [x] 2.3 Integration tests (testcontainers): fitted-model happy path, sprint/60m exclusion, insufficient-points, span-too-narrow, empty window, full 400 matrix, read-only (no row mutation), unit-isolation (`assert.NotContains` nutrition/hydration fields)
- [x] 2.4 `task swag`

## 3. MCP

- [x] 3.1 `cp_model` read-tier tool: one GET, verbatim body; description carries the advisory CP≈FTP framing + pointer to the athlete-config update flow for applying thresholds
- [x] 3.2 Golden regen (`-tags=goldengen`, additive) + registry/integration tests green

## 4. Dashboard panel

- [x] 4.1 `useCPModel` hook + TS types mirroring the response shape
- [x] 4.2 `/stats` panel: CP/W′/R² readout, points + fitted hyperbola on log-x (visx, power-curve panel conventions), window selector, null-model degraded state naming the reason
- [x] 4.3 Web tests (fitted render, degraded render, selector) — `tsc` + vitest green

## 5. Verification

- [x] 5.1 `task vet` + full Go suite green (re-run isolated `-p 1` on any testcontainers boot-contention flake)
- [ ] 5.2 Live sanity check against real data: fit over the trailing 90 days, eyeball CP vs configured FTP and the per-point contributions _**(manual — needs live data; not runnable from this environment)**_
