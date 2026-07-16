# Tasks

## 1. Backend

- [x] 1.1 `internal/expenditure/`: pure balance math (logged-day rule, trend-endpoint delta, gates) + unit tests (hand-computed fixture, unlogged-day exclusion, both gates, boundary rounding, negative/positive trend deltas)
- [x] 1.2 Narrow read interfaces (meals per-day kcal + logged-day detection; bodyweight trend at window ends with weigh-in count) wired in `httpserver.Run()`
- [x] 1.3 `GET /nutrition/expenditure` handler: 92-day cap + shared range vocabulary; integration tests (estimate happy path, unlogged counting, gate matrix, empty window, read-only, no goals/config coupling)
- [x] 1.4 `task swag`

## 2. MCP

- [x] 2.1 `energy_expenditure` read tool (window guidance + bias caveats in description); golden regen (additive) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 3.2 Live sanity: trailing-28-day read against real logging; eyeball vs the current goal kcal and known weight trend
      _(operator step — needs the live MCP client against real logged data; not runnable in-session)_
