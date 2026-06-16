## 1. Multisport template model

- [x] 1.1 Verify the next migration number (head `041` on disk — confirm before scaffolding) and create a `multisport_templates` table (name + segments JSONB).
- [x] 1.2 New `internal/multisport/` package: `types.go` (Template → ordered `[]Segment{sport, steps[], transition duration}`), `repo.go`, `service.go`, `handlers.go`.
- [x] 1.3 Validate: ≥2 non-transition segments; each non-transition segment's steps validated by the existing workout-templates step validator **under that segment's sport**; transition segments carry only a duration.

## 2. REST + MCP surface

- [x] 2.1 Register REST CRUD in `internal/httpserver/server.go`.
- [x] 2.2 New `internal/agenttools/` tool group mirroring the REST surface 1:1; bump the MCP integration expected-tools list.

## 3. Garmin bridge multi-segment compile

- [x] 3.1 Extend `workout_builder.build_payload` with a multisport branch: one `workoutSegments` entry per segment with its own `sportType`, monotonic `segmentOrder`, whole-workout step-order counter.
- [x] 3.2 Map `transition` segments to Garmin's transition sport type. Verify exact sport-type ids and segment field names against garminconnect. _**VERIFIED LIVE 2026-06-16** via create+read round-trips through the running bridge (Garmin normalizes sportTypeId→canonical key on store). **`multi_sport` = id 10** — a first guess of 9 stored as `"hiit"` (would have pushed the tri as a HIIT workout); top-level corrected 9→10 and a full swim/T1/bike/T2/run probe read back top-level `multi_sport` with segments swim(4)/bike(2)/run(1). An exhaustive id probe (0, 8, 14–50) then proved there is **no transition workout sportType at all**: the complete valid workout-service vocabulary is ids 1–13 (…, 8=pilates, 10=multi_sport, …); id 0 and every id ≥14 store as sportTypeId 0 (no sport), and none normalizes to "transition" — it's only an *activity* type. So `multi_sport(10)` is the correct representation for T1/T2 segments; there is nothing further to find. Probe workouts created + deleted; no residue._
- [x] 3.3 Unit-test the multi-segment payload (swim/T1/bike/T2/run → 5 ordered segments, monotonic step order).

## 4. Compile + schedule action

- [x] 4.1 Add a Go path (`internal/garmincontrol/`) that compiles a multisport template via the bridge and schedules it to a date, reusing existing calendar mechanics; return the Garmin workout id.

## 5. Per-segment composition (gated on prior changes)

- [x] 5.1 If `resolve-zone-targets` has shipped: run its zone resolver over each segment's steps keyed by the segment's sport (bike resolves power_zone; run/swim pass through). Add a per-segment resolution scenario.
- [x] 5.2 If `add-secondary-target` has shipped: allow bike segments to carry secondary targets and ensure the bridge emits them per segment.

## 6. Tests + docs

- [x] 6.1 Integration test: create a triathlon multisport template; assert per-sport validation (swim `pace` rejected, run `secondary_target` rejected, <2 sport segments rejected, transition-with-steps rejected).
- [x] 6.2 Integration test (or mocked bridge): schedule action pushes one multisport workout and returns an id.
- [x] 6.3 Run `task swag`, `task test`, `task vet`.
