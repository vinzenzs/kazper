# Tasks

## 1. Ranking model (pure, unit-tested)

- [x] 1.1 `internal/effortanalytics/powerprofile.go`: embedded Coggan tables (four columns × N category rows × male/female, from Coggan/Allen), `rankAnchor(wPerKg, column, sex) → (category, percentile)` (row lookup + linear interpolation, clamped `[0,100]`), `phenotype(anchors) → *string` (relative-percentile thresholds, nil unless all four present); no I/O
- [x] 1.2 Unit tests: exact published anchor W/kg → expected category (spot-check several cells per sex), interpolation monotonic + clamped at both ends, each phenotype branch (sprinter/time_trialist/climber/all_rounder) + nil-on-incomplete, FT-proxy uses the threshold column with no 0.95 haircut

## 2. Endpoint

- [x] 2.1 Types (`PowerProfileAnchor`, `PowerProfileResult` with `weight_kg`/`weight_source`/`sex`/`anchors`/`missing_anchors`/`phenotype`); `weightProvider` interface (`LatestWeight(ctx) (float64, error)`); service `PowerProfileFor` reusing `CurveFor` (power) → pick the four anchor durations → resolve weight → rank; `numfmt` (watts int, w_per_kg + percentile `Round1`)
- [x] 2.2 `GET /workouts/power-profile` handler sharing `parseWindow`: `weight_kg` (`weight_kg_invalid`), `sex` default male (`sex_invalid`), `weight_data_missing`; swag annotations; wire the `weightProvider` (bodyweight repo) in `httpserver.Run()`
- [x] 2.3 Integration tests (testcontainers): full four-anchor rank, missing-anchor omitted + listed, weight param vs stored fallback vs weight_data_missing, sex_invalid, weight_kg_invalid, window 400 matrix, no-mutation, phenotype present/null
- [x] 2.4 `task swag`

## 3. MCP

- [x] 3.1 `power_profile` read tool (one GET; args `from`/`to`/`tz`/`weight_kg`/`sex`; description: four anchors + 20-min FT proxy + advisory category-primary framing)
- [x] 3.2 Golden regen (`-tags=goldengen`, additive) + registry/integration tests green

## 4. Dashboard panel

- [x] 4.1 `usePowerProfile` hook + TS types
- [x] 4.2 `/stats` power-profile panel: per-anchor W/kg + category badge + percentile, phenotype label when present, degraded/empty state on `weight_data_missing`/no-anchors; own window selector consistent with the CP/curve panels
- [x] 4.3 Web tests (renders ranked anchors, degraded state, phenotype present/absent) — `tsc` + vitest green

## 5. Docs & verification

- [x] 5.1 README MCP table row for `power_profile`
- [x] 5.2 `task vet` + full Go suite green (isolated `-p 1` rerun on testcontainers boot-contention flakes)
- [ ] 5.3 Live sanity: rank a real window against the tables, eyeball the categories + phenotype against known ability
