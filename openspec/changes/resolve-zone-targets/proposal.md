## Why

The athlete's physiology config (FTP, power zones, HR zones, threshold paces) is stored but consumed by nothing — `athlete-config` is capture-only. Meanwhile every workout-template step carries `target:{kind:"none"}` or, where targets exist, zone references (`power_zone:4`) that the Garmin watch resolves against **Garmin Connect's** zones, not ours. The result: the seeded FTP/zones drive zero prescribed intensity, and a "Race Pace" workout pushed to the watch is a name with no gate. With the race ~6 weeks out, wiring config into prescribed targets is the highest-leverage fix before Peak/Taper.

## What Changes

- Make `EffectiveProgram` (the single chokepoint feeding both `get_workout_program` and the Garmin push) **resolve zone-reference targets to absolute values** using the athlete config singleton:
  - `power_zone:N` → `power_w` range `[PowerZone{N-1}Max .. PowerZone{N}Max]`
  - `hr_zone:N` → `hr_bpm` range `[HRZone{N-1}Max .. HRZone{N}Max]`
  - Zone ranges (`low≠high`) span from the low zone's lower edge to the high zone's upper edge.
  - `pace`, `power_w`, `hr_bpm`, `rpe`, `none` pass through unchanged.
- **Graceful fallback**: when athlete config is absent or the required zone boundary is unset, the zone-reference passes through **unchanged** (Garmin resolves it from Connect). Resolution never errors a push.
- **Preserve provenance**: a resolved step retains a human-readable origin label (e.g. `"Z4"`) so the coach/agent sees both `230–268W` and where it came from in `get_workout_program`.
- Wire the athlete-config repo into `trainingplan.Service` via cross-injection in `httpserver.go` (mirrors the existing `SetWorkoutsRepo` pattern).
- No garmin-bridge changes — it already emits `power_w`/`hr_bpm` absolute targets.
- Swim pace targets (`sec_per_100m`) are explicitly **out of scope** (separate proposal); `pace` stays passthrough.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `training-plan`: `EffectiveProgram` gains a target-resolution pass that expands zone-reference targets into absolute `power_w`/`hr_bpm` ranges from athlete config, with graceful passthrough on missing config and origin provenance on resolved steps.
- `athlete-config`: no longer capture-only — its zone boundaries are now consumed as the single source of truth for resolving workout step targets.

## Impact

- **Code**: `internal/trainingplan/` (new resolver + `Service` wiring), `internal/httpserver/server.go` (cross-injection). New dependency edge `trainingplan → athleteconfig` (athleteconfig is a leaf — no import cycle).
- **Behavior**: `get_workout_program` output for steps with zone targets now shows absolute numbers + origin label; Garmin-pushed workouts get real power/HR gates derived from our config.
- **Out of scope**: garmin-bridge (unchanged), swim pace targets, and the data back-fill of ~45 templates with zone-refs (separate coaching task, works independently and even before this lands).
