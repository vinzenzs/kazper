## 1. Resolver core

- [x] 1.1 Add a `resolveTargets(steps []workouttemplates.Step, cfg *athleteconfig.AthleteConfig) []workouttemplates.Step` pass in `internal/trainingplan/` that walks the step tree (including nested `repeat` steps) and rewrites zone targets.
- [x] 1.2 Implement zone→absolute math: `power_zone[lo,hi]` → `power_w[PowerZone{lo-1}Max .. PowerZone{hi}Max]` (lo=1 → low 0); `hr_zone[lo,hi]` → `hr_bpm[HRZone{lo-1}Max .. HRZone{hi}Max]`. Flip the target `kind` accordingly.
- [x] 1.3 Implement graceful passthrough: nil config, nil repo, or any unset required `*Max` boundary leaves the zone target unchanged and never errors.
- [x] 1.4 Pass through `pace`, `power_w`, `hr_bpm`, `rpe`, `none` unchanged (no double-resolution of hand-typed absolutes).
- [x] 1.5 Attach an `origin` label (e.g. `"Z4"`, `"Z2–Z4"`) to each resolved target; add the optional `omitempty` field to the program step target shape.

## 2. Wiring

- [x] 2.1 Add `SetAthleteConfigRepo(...)` setter to `trainingplan.Service` (optional dependency, nil-safe).
- [x] 2.2 Call the setter in `internal/httpserver/server.go` `Run()` alongside the existing cross-injection block.
- [x] 2.3 Invoke `resolveTargets` at the end of `EffectiveProgram` (after `applyOverrides`), loading the athlete-config singleton via the injected repo.

## 3. Tests

- [x] 3.1 Unit-test `resolveTargets`: single-zone power, single-zone HR, multi-zone band, zone-1 floor of 0, nested-in-repeat, and passthrough kinds.
- [x] 3.2 Integration-test `GET /workouts/{id}/program`: zone target resolves to absolute range with `origin`; missing-config case leaves the zone target unchanged.
- [x] 3.3 Verify the Garmin push path (`pushOne` → `bridgeCreateWorkout`) sends resolved `power_w`/`hr_bpm` for a zone-targeted workout (existing scheduling test or a new assertion).

## 4. Docs & surface

- [x] 4.1 Update `get_workout_program` MCP tool description to note resolved absolute targets + `origin` label.
- [x] 4.2 Run `task swag` to regenerate `docs/` for the `/workouts/{id}/program` response shape change.
- [x] 4.3 Run `task test` and `task vet`; confirm the MCP integration expected-tools list still passes (no new tools added).
