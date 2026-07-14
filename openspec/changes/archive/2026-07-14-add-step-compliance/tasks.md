# Tasks — add-step-compliance

## 1. Package: internal/workoutcompliance

- [x] 1.1 Create `internal/workoutcompliance/types.go`: response types —
  `Result { WorkoutID, TemplateID, Status ("scored"|"unavailable"), Reason *string, PlannedSteps, ExecutedLaps int, Score *float64, StepsScored, StepsInBand int, Steps []StepResult }`,
  `StepResult { StepIndex, Intent, Iteration/Of (repeat provenance, omitempty), Planned (duration + resolved target incl. Origin), Actual (lap metrics), Target *TargetResult, Secondary *TargetResult, Duration *DurationResult, Score *float64, Unscorable reason fields }`,
  with omitempty on all nullables per convention.
- [x] 1.2 Add pure expansion in `service.go`: flatten the effective program's step
  tree (repeat `count` → consecutive copies, one level deep) carrying flat
  `step_index` + repeat provenance; unit-testable without a DB.
- [x] 1.3 Add pure scoring functions: metric selection by resolved target kind
  (power_w→AvgPowerW, hr_bpm→AvgHR, pace→1000/AvgSpeedMPS, swim_pace→100/AvgSpeedMPS;
  cadence/rpe/none/unresolved-zone/missing-actual → unscorable with reason),
  in_band/under/over classification with signed `delta` + `deviation_pct`,
  target score (100 in band, linear falloff to 0 at 25% out), duration score for
  time/distance steps (±10% in-band, falloff to 0 at ±25%; lap_button/open
  unscored), step score (0.7 target + 0.3 duration when both), overall
  planned-duration-weighted mean + steps_scored/steps_in_band, null score when
  nothing scorable. Tolerances as package constants.
- [x] 1.4 Add `Service` with narrow injected interfaces (`workoutsRepo.GetByID`,
  `programProvider.EffectiveProgram`) and sentinel errors:
  `ErrNotCompleted`, `ErrMultisportUnsupported`, `ErrNoTemplateLink`,
  `ErrSplitsMissing` (multisport check before template-link check); strict
  `len(splits) == len(expanded)` gate returning the `unavailable` result on
  mismatch.
- [x] 1.5 Add `handlers.go`: `GET /workouts/:id/compliance` with swag annotations
  (`workoutfueling` registration pattern), mapping sentinels 1:1 —
  400 `workout_id_invalid`, 404 `not_found`, 409 `workout_not_completed` /
  `multisport_unsupported` / `no_template_link` / `splits_missing` — and
  `numfmt.Round1` on all numeric response fields.

## 2. Wiring

- [x] 2.1 Instantiate and register in `internal/httpserver/server.go` `Run()`:
  `workoutcompliance.NewService(workoutsRepo, trainingPlanSvc)` +
  `NewHandlers(...).Register(api)` (after `trainingPlanSvc` construction, since
  it is the program provider).

## 3. MCP tool

- [x] 3.1 Add a `workout_compliance` registry entry in `internal/agenttools/`
  (alongside `workout_adherence` in `registry_workouts.go`): args
  `{workout_id}`, `TierRead`, one `GET /workouts/{id}/compliance`, description
  covering the scored/unavailable shapes and error codes.
- [x] 3.2 Regenerate the announced-schema golden for the new tool:
  `go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/`
  (no manual expected-tools bump — `AnnouncedToolNames` derives from the
  registry) and confirm `internal/mcpserver/mcp_integration_test.go` +
  `schema_golden_test.go` pass with the tool announced.

## 4. Tests

- [x] 4.1 Unit-test expansion: single steps, `repeat ×N` ordering + provenance
  (interval 3 of 5 nameable), mixed programs; property: expanded length =
  Σ(singles) + Σ(count × group size).
- [x] 4.2 Unit-test scoring tables: under/over/in-band per kind incl. the
  power `delta: -20` case, pace derivation from AvgSpeedMPS (and swim_pace),
  unresolved-zone/cadence/rpe/none/missing-actual unscorable reasons, duration
  ratio + lap_button/open skip, boundary values at the tolerance constants,
  weighted overall score (600s@60 + 60s@100 ≈ 63.6), null score when nothing
  scorable.
- [x] 4.3 Handler integration tests (testcontainers, per-handler convention):
  happy path scoring a template-linked completed workout with matching splits;
  slot target_overrides shaping the compared band; repeat-expansion end-to-end
  (12 splits vs warmup+5×(interval,recovery)+cooldown); lap-count mismatch →
  200 unavailable with counts; each error path (unknown id → 404, planned →
  409, multisport → 409 multisport_unsupported not no_template_link, no
  template → 409, zero splits → 409); compute-on-read (no row mutated).
- [x] 4.4 Verify the MCP e2e/golden suite covers the new tool announcement
  (`go test -count=1 -tags=integration ./internal/mcpserver/...`).

## 5. Docs & verification

- [x] 5.1 Run `task swag` to regenerate `docs/` for the new endpoint + response
  types.
- [x] 5.2 Run `task vet` and `go test -count=1 ./internal/workoutcompliance/...
  ./internal/agenttools/... ./internal/mcpserver/...`, then the full
  `task test`; re-run any testcontainers parallel-boot flakes isolated with
  `-p 1`.
