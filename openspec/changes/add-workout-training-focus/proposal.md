## Why

A workout row records *what* happened (sport, duration, HR, power, TSS) but not the
*training intent* behind it. For endurance training the intensity band is the primary
lens: a 90-min Z2 base ride and a 90-min threshold session are nutritionally and
physiologically different sessions that today look identical to the system. The
established taxonomy for this is the German Trainingsbereiche model
(GA1/GA2/WSA/…). Without it, adherence review, fueling recommendations, and
plan/phase analytics cannot distinguish basic-endurance volume from
competition-specific work.

## What Changes

- Add an optional `training_focus` enum field to a workout, classifying the session's
  intensity band against the 7-zone Trainingsbereiche model:
  `recovery` (REKOM), `basic_endurance_1` (GA1), `basic_endurance_2` (GA2),
  `development` (EB), `competition_specific` (WSA), `peak` (SB),
  `strength_endurance` (KA).
- The field is nullable (sessions without a classification stay NULL — "unclassified"
  is a valid state, matching the rpe/ingestion-metric precedent), settable on
  `POST /workouts` (and `/workouts/bulk`) and on `PATCH /workouts/{id}` with the
  established tri-state semantics (absent = unchanged, value = set, JSON `null` = clear).
- Returned on `GET /workouts` and `GET /workouts/{id}` via the `omitempty` pattern.
- Validated against the enum with a new `training_focus_invalid` error code, mirroring
  how `sport`/`status`/`source` are validated.
- DB column added via a new migration with a CHECK constraint and no back-fill.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workouts`: a new optional `training_focus` enum field on the workout row, accepted
  and validated on POST / bulk / PATCH, returned on GET; nullable with no back-fill.

## Impact

- **Schema**: new migration `048_add_workout_training_focus.{up,down}.sql` — adds a
  nullable `training_focus TEXT` column to `workouts` with a CHECK constraint over the
  7 allowed values.
- **Code**: `internal/workouts/{types,service,repo,handlers}.go` — new `TrainingFocus`
  type + `ValidTrainingFocus`/`ParseTrainingFocus`, `ErrTrainingFocusInvalid` sentinel,
  field threaded through `Workout`, `CreateInput`, `PatchParams`/`PatchInput`,
  `selectCols`, Upsert INSERT/UPDATE, and the PATCH dynamic SET builder.
- **Docs**: `task swag` regenerates `docs/` from the updated request/response structs.
- **MCP**: no new tools; workouts are not yet a dedicated MCP write surface, so the
  field rides along once the REST struct carries it. No `mcp_integration_test`
  expected-tools change.
- **Mobile companion** (`apps/companion`): optional follow-up — `TrainSession` can
  surface `training_focus` once present on the wire; not required for this change and
  out of scope here.
