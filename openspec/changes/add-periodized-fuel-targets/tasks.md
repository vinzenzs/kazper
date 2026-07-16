# Tasks

## 1. Backend

- [ ] 1.1 `internal/fuelplan/`: pure classifier (tier thresholds + long-session rule + g/kg ladder) + unit tests (threshold boundaries, duration rule, multi-session summing, plan-missing flag, weight-missing degradation, rounding)
- [ ] 1.2 Narrow read interfaces (planned workouts in range; bodyweight trend latest; effective goal carbs per date incl. overrides) wired in `httpserver.Run()`
- [ ] 1.3 `GET /nutrition/fuel-plan` handler (default window today+6, 14-day cap, shared range vocabulary); integration tests (classified week, heavy/long rules, plan-missing tail, weight-missing, override-aware effective goals, read-only/no-writes)
- [ ] 1.4 `task swag`

## 2. Context & MCP

- [ ] 2.1 `/context/daily` `fuel_plan` block (today + tomorrow, omitted when uncomputable) + tests
- [ ] 2.2 `fuel_plan` read tool (override-flow + carbs-within-kcal framing in description); golden regen (additive) + registry/integration green

## 3. Verification

- [ ] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 3.2 Live sanity: classify the current real training week; eyeball tiers against the actual plan and the suggested grams against current goals
