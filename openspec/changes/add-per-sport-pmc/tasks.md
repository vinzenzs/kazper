# Tasks

## 1. Backend

- [x] 1.1 `internal/pmc` repo queries gain the optional sport predicate (per-day SUM + earliest-date, both filtered); handler validates against the workouts sport vocabulary (`sport_invalid`), echoes `sport`
- [x] 1.2 Tests: run-filtered series vs hand-computed fixture, per-sport `seed_date`, omitted-param regression (byte-identical combined response), invalid sport, multisport-as-own-sport
- [x] 1.3 `task swag`

## 2. MCP

- [x] 2.1 `pmc_series` gains optional `sport` arg + description note (combined ≠ sum); golden regen (input-schema touch) + registry/integration green

## 3. Dashboard

- [x] 3.1 Sport selector on the PMC panel (All default, target overlay All-only, filtered-failure fallback); web tests — `tsc` + vitest green

## 4. Verification

- [x] 4.1 `task vet` + full suite green (`-p 1` on boot-contention flakes); live: bike vs run vs combined eyeball check
