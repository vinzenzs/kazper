# Tasks

## 1. Model math (pure, unit-tested)

- [ ] 1.1 `internal/activitystreams/wprimebal.go`: differential FCS recursion over a 1 Hz power slice given (cp_watts, w_prime_j) → series + summary (min + at, end, max depletion %, time below 25%); no clamp at zero; no I/O
- [ ] 1.2 Unit tests: constant supra-CP linear depletion (hand-computed), depletion→recovery round trip (recharge approaches W′ asymptotically, never exceeds), negative-floor case (min < 0, depletion > 100%), all-sub-CP ride (flat at W′), time-below-25% accounting, boundary rounding only

## 2. Endpoint

- [ ] 2.1 `GET /workouts/{id}/w-prime-balance` handler in `internal/activitystreams/`: reuse the stored-stream read; param validation (`cp_invalid`/`w_prime_invalid`), `power_stream_missing` sentinel, `summary_only`, series `downsample` via the existing bucket-mean helper (bounds + echo + `downsample_invalid`)
- [ ] 2.2 Integration tests (testcontainers): happy path (summary + full series), `summary_only`, downsample applied + bounds, param 400 matrix, `workout_not_found` / `streams_not_found` / `power_stream_missing` distinction, nothing persisted (no row mutation), no athlete-config dependency
- [ ] 2.3 `task swag`

## 3. MCP

- [ ] 3.1 `w_prime_balance` read-tier tool: one GET with `summary_only=true` hardcoded; args `workout_id`/`cp_watts`/`w_prime_kj`; description points at `cp_model` as the parameter source + advisory framing
- [ ] 3.2 Golden regen (`-tags=goldengen`, additive) + registry/integration tests green

## 4. Dashboard strip

- [ ] 4.1 `useWPrimeBalance` hook + TS types; share the cp-model fetch/hook with the stats panel
- [ ] 4.2 `/workouts/:id` W′bal strip (visx series with min marker + summary readout), absent when cp-model null / no power stream / fetch error; rest of the page unaffected
- [ ] 4.3 Web tests (renders with prerequisites, absent without, summary values) — `tsc` + vitest green

## 5. Verification

- [ ] 5.1 `task vet` + full Go suite green (isolated `-p 1` rerun on testcontainers boot-contention flakes)
- [ ] 5.2 Live sanity check: pick a recent hard interval ride, feed the current cp-model values, eyeball the depletion profile against how the session actually felt
