# Tasks

## 1. Backend

- [x] 1.1 `missed_limit` param (bounds, default 50, truncation-flag semantics) + `zero_fill` serialization-time fill (calendar span / plan ordinals, zeroed counts, null rate) — `computeAdherence` fold untouched
- [x] 1.2 Tests: raised-limit list, omission regression, bounds 400s, zero-fill calendar + plan-ordinal continuity, populated-bucket invariance, totals invariance
- [x] 1.3 `task swag`

## 2. MCP

- [x] 2.1 `workout_adherence` forwards both optional args; golden regen (input-schema touch) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + `internal/workouts` suite green; live YTD read with `missed_limit=200&zero_fill=true`
