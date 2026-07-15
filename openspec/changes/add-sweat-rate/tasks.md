# Tasks

## 1. Backend

- [x] 1.1 Pure computation in `internal/workoutfueling/` (loss/rate/itemization/override/warning band); unit tests (worked fixture, override replacement, negative-loss + >5 L/hr warnings, rounding)
- [x] 1.2 Handler `GET /workouts/{id}/sweat-rate` over the existing three repos: param matrix, `workout_not_completed`, `not_found`; verify the hydration/workout-fuel linked-ml projections cover the sum (widen if needed)
- [x] 1.3 Integration tests: full field test with linked entries, override, warnings, 409/404, param 400s, read-only, unit-isolation (`NotContains` kcal/sodium in response)
- [x] 1.4 `task swag`

## 2. MCP

- [x] 2.1 `sweat_rate` read tool; golden regen (additive) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + workoutfueling suite green; live: run the numbers on a recent long ride with real logged bottles
