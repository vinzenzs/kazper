## Context

`trainingplan.Service.EffectiveProgram` (`internal/trainingplan/service.go:457`) is the single point where a planned workout's steps are finalized: it loads the template, applies slot `TargetOverrides`/`DurationOverrides` via `applyOverrides`, and returns a `Program`. Both consumers go through it:

- **Coach/agent**: `get_workout_program` MCP tool → `EffectiveProgram`.
- **Watch**: `garmincontrol.pushOne` (`scheduling.go:348`) → `EffectiveProgram` → `bridgeCreateWorkout` → garmin-bridge `POST /workouts`.

Today targets are zone references (`{kind:"power_zone", low:4, high:4}`) or `none`. The garmin-bridge emits zone targets as `zoneNumber`, which the watch resolves against **Garmin Connect's** zones — our `athlete_config` (FTP/zones) is never read. `athlete-config` is documented "capture only".

Zone boundaries live in the `athleteconfig` singleton as per-zone *Max* values: `PowerZone1Max..PowerZone5Max`, `HRZone1Max..HRZone5Max` (`internal/athleteconfig/types.go`). The `Target` struct already supports absolute kinds `power_w` and `hr_bpm` with `Low`/`High` ints, and the garmin-bridge already maps those to `targetValueOne/Two`.

## Goals / Non-Goals

**Goals:**
- Resolve zone-reference targets to absolute `power_w`/`hr_bpm` ranges from `athlete_config`, computed fresh on every `EffectiveProgram` call (so a config edit retargets all workouts with no template edits).
- Cover both consumers (watch + `get_workout_program`) from one insertion point.
- Never break a push: degrade to passthrough when config is missing/incomplete.
- Preserve provenance so the coach sees both the absolute numbers and the source zone.

**Non-Goals:**
- Swim pace targets (`sec_per_100m`) — separate proposal; `pace` stays passthrough.
- Pushing our zones to Garmin Connect.
- Back-filling the ~45 templates with zone-refs (independent coaching/data task).
- Per-sport zone tables (config has one HR-zone set and one power-zone set).
- Changing the garmin-bridge.
- **Cadence targets** — Garmin exposes a cadence target kind (bike rpm / run spm) we don't model; deferred follow-up (new cross-sport `Target.kind`).
- **Secondary targets (bike only)** — Garmin bike steps carry Primary + Secondary targets (e.g. Power Zone *and* Cadence); run steps do not. Our `Step.Target` is a single slot. Adding a second (bike-scoped) target slot is a structural model change, deferred follow-up.
- **Multisport structured workouts** — a single pushed workout with multiple sport segments + transitions (swim→T1→bike→T2→run). Not modeled (bricks are separate single-sport rows linked by `session_group`); the garmin-bridge builder emits one segment only. Larger follow-up; would require per-segment sport on steps and break the D7 single-sport assumption.

## Decisions

### D1: Resolve inside `EffectiveProgram`, after `applyOverrides`
Add a final pass `resolveTargets(steps, cfg)` that walks the resolved step tree (including nested `repeat` steps) and rewrites each step's `Target`. Chosen over resolving in the garmin-bridge (would miss `get_workout_program` and duplicate athlete-config access in Python) and over baking absolutes at authoring time (defeats the live-engine property — numbers would go stale on FTP change).

### D2: Zone → absolute math
For `power_zone` with zones `[lo, hi]` (1–5):
- `power_w.Low  = PowerZone{lo-1}Max` (lo=1 → `0`)
- `power_w.High = PowerZone{hi}Max`

Identical shape for `hr_zone` → `hr_bpm` using `HRZone{N}Max`. Single zone (`lo==hi`) yields the band `[Zone{N-1}Max .. Zone{N}Max]`. Zone-1 lower edge is `0` (no lower gate) — simplest defensible floor; config has no sub-zone-1 boundary. The kind flips (`power_zone`→`power_w`, `hr_zone`→`hr_bpm`) so the garmin-bridge emits a value range, not a `zoneNumber`.

### D3: Graceful passthrough on missing data
A zone target passes through **unchanged** (resolver returns it as-is, so the bridge emits `zoneNumber` and the watch uses Garmin Connect) when: athlete_config is null, or any required `*Max` boundary for the requested zone(s) is nil. Resolution is best-effort and never returns an error from this path. Alternative (error/refuse push) rejected — would make the watch worse than today before config is fully seeded.

### D4: Provenance via a non-breaking origin field
The resolved step records where its absolute target came from (e.g. origin `"Z4"` / `"HR Z2–Z3"`). Implementation: add an optional `origin`/`resolved_from` string to the program step's target (omitempty, additive — no breaking change to the JSON contract). `get_workout_program` then shows `230–268W (Z4)`. The garmin-bridge ignores the field.

### D5: Wiring via cross-injection
`trainingplan.Service` gets a `SetAthleteConfigRepo(...)` setter, called in `httpserver.Run()` alongside the existing `mealsSvc.SetWorkoutsRepo(...)` cross-injection. Keeps the dependency optional and the wiring in the trunk. `athleteconfig` imports nothing from `trainingplan`, so no import cycle. If the repo is unset (defensive), resolution behaves as D3 (passthrough).

### D7: Power-zone resolution is gated to bike
`athlete_config` stores a single power-zone set (FTP/bike-derived). Running power (Stryd-style) runs on a different threshold, so expanding a *run* `power_zone` against bike watts would emit wrong numbers on the watch. The resolver SHALL resolve `power_zone` only for bike workouts; for any other sport a `power_zone` target passes through unchanged (D3 fallback). `hr_zone` is **not** gated — HR zones are max-HR-derived and apply across sports. This guard is the in-change consequence of the per-sport-zone-tables non-goal; revisit if run power becomes a real prescription.

The gate is unambiguous because every workout/template is **strictly single-sport** (`Program.Sport` is one value; the model has no multisport structured workout — bricks/triathlons are decomposed into separate single-sport workout rows linked by `session_group`). So a brick's bike leg and run leg are two workouts resolved independently: the bike leg resolves `power_zone`, the run leg passes it through. No per-step sport lookup is needed. (A true multisport *structured* workout — multiple sport segments in one pushed workout — would break this assumption and require per-segment sport on steps; it is a separate, larger follow-up — see Non-Goals.)

### D6: Resolution is read-time only
Nothing is persisted. Templates and slots keep zone-refs; `EffectiveProgram` is pure `(template, overrides, config) → resolved program`. This is what gives "edit FTP once → re-push → new gates".

## Risks / Trade-offs

- **Garmin Connect zones disagree with our config** → Before this change the watch silently used Connect's zones; after, resolved workouts use *our* numbers (the intent). Mitigation: provenance label makes the source visible; passthrough still defers to Connect when config is incomplete.
- **Zone-1 floor of 0 looks odd on HR** (e.g. `0–142 bpm`) → Acceptable: zone-1/recovery steps rarely need a hard lower HR gate; revisit only if it proves annoying.
- **Existing absolute targets** (`power_w`/`hr_bpm` typed by hand) → Untouched by the resolver (passthrough), so no double-resolution.
- **`get_workout_program` output shape changes** for zone steps (kind flips, origin appears) → Additive field + a documented behavior change; the spec delta captures it. No consumer parses the old `power_zone` kind from this endpoint today.

## Migration Plan

Pure code change, no DB migration. Deploy: ship resolver + wiring; existing zone-ref data resolves automatically on next `get_workout_program`/re-schedule. Rollback: revert the commit — `EffectiveProgram` returns to passthrough, zone-refs again resolved by the watch. No data to undo.

## Open Questions

- Zone-1 lower edge: `0` (chosen) vs. a fraction of max (e.g. 50% MaxHR for HR). Start with `0`; cheap to revisit.
- Should `get_workout_program` also surface the zone boundaries it used (for coach explainability) beyond the per-step origin label? Defer unless asked.
