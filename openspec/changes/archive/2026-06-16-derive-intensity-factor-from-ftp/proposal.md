## Why

`athlete_config.ftp_watts` and a workout's `normalized_power_w` are both captured today, but the one value they trivially produce — cycling Intensity Factor (`IF = NP / FTP`) — is never derived. The `athlete-config` spec explicitly deferred this consumption ("Storing FTP does not back-fill workout intensity_factor"). The zone-target consumptions that share that deferral (cadence, secondary target, swim pace, multisport) have all shipped; IF-from-FTP is the last named follow-up. Without it, bike workouts ingested without a watch-supplied IF (manual logs, activities Garmin returns NP-but-no-IF for) sit with `intensity_factor` NULL even though every input to compute it is present.

## What Changes

- The `workouts` service derives `intensity_factor = normalized_power_w / ftp_watts` (rounded to 2dp) at **create and update time**, storing it into the existing `intensity_factor` column.
- Derivation is **gated**: it fires only when the sport is `bike`, `normalized_power_w` is present and `> 0`, the athlete-config singleton has `ftp_watts > 0`, and the caller did **not** explicitly supply `intensity_factor` (a watch-/client-provided IF always wins — derivation only fills the gap).
- The `workouts` service gains a read-only cross-injected dependency on the `athlete-config` repo, wired in `httpserver/server.go` via a `SetAthleteConfigRepo(...)` optional-setter (mirroring `trainingPlanSvc.SetAthleteConfigRepo` and `mealsSvc.SetWorkoutsRepo`). When the dependency is absent (or FTP unset), the workout writes through unchanged — no derivation, no error.
- No bulk back-fill migration: existing NULL rows fill naturally the next time they are re-synced or patched. IF is therefore computed against the FTP **in effect at write time** (point-in-time correct), not retroactively rewritten when FTP changes.
- The `athlete-config` spec's deferral scenario is reversed: storing FTP (and writing a qualifying bike workout) now **does** produce an `intensity_factor`.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workouts`: ADD a requirement that the service derives and stores `intensity_factor` from `normalized_power_w / ftp_watts` on create/update, under the bike-only / NP-present / FTP-configured / IF-not-supplied gate, never overriding a caller-supplied IF.
- `athlete-config`: MODIFY the "Config is the capture-only source of physiology" requirement to remove the IF-from-FTP carve-out and replace the "Storing FTP does not back-fill workout intensity_factor" scenario with one asserting a qualifying bike workout now receives a derived IF (the singleton remaining otherwise-unconsumed for the other deferred items).

## Impact

- **Code**: `internal/workouts/service.go` (derivation + gate, new optional setter + repo field), `internal/workouts/types.go`/`service.go` input plumbing if needed for the supplied-vs-derived distinction; `internal/httpserver/server.go` (cross-inject `athleteConfigRepo` into `workoutsSvc`). New unit/integration tests in `internal/workouts`.
- **APIs**: No new endpoints and no request-shape change. `GET /workouts/{id}` and list responses may now return a non-NULL `intensity_factor` for bike workouts that previously returned NULL. `task swag` only if annotations change (not expected).
- **Dependencies**: `workouts` → `athleteconfig` (read-only, repo-level). No import cycle: `athleteconfig` does not import `workouts`.
- **MCP**: No tool changes — the field already passes through verbatim.
