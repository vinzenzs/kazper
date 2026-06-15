## 1. Multisport template model

- [x] 1.1 Verify the next migration number (head `041` on disk — confirm before scaffolding) and create a `multisport_templates` table (name + segments JSONB).
- [x] 1.2 New `internal/multisport/` package: `types.go` (Template → ordered `[]Segment{sport, steps[], transition duration}`), `repo.go`, `service.go`, `handlers.go`.
- [x] 1.3 Validate: ≥2 non-transition segments; each non-transition segment's steps validated by the existing workout-templates step validator **under that segment's sport**; transition segments carry only a duration.

## 2. REST + MCP surface

- [x] 2.1 Register REST CRUD in `internal/httpserver/server.go`.
- [x] 2.2 New `internal/agenttools/` tool group mirroring the REST surface 1:1; bump the MCP integration expected-tools list.

## 3. Garmin bridge multi-segment compile

- [x] 3.1 Extend `workout_builder.build_payload` with a multisport branch: one `workoutSegments` entry per segment with its own `sportType`, monotonic `segmentOrder`, whole-workout step-order counter.
- [x] 3.2 Map `transition` segments to Garmin's transition sport type. Verify exact sport-type ids and segment field names against garminconnect. _Structure + per-segment ids (from the verified `_SPORT` map) done. The top-level `multi_sport` and `transition` workout-service sportTypeIds are flagged `TO VERIFY` in `workout_builder.py` — the garminconnect lib's `SportType` enum uses non-workout-service numbering, so a wrong id is silently stored as 0; needs one on-device push to confirm before relying on the watch._
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
