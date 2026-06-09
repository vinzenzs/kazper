## 1. Shared body-weight resolver — hoist into bodyweight package

- [x] 1.1 Create `internal/bodyweight/resolve.go`: exported `func ResolveAtDate(ctx context.Context, repo *Repo, date time.Time, loc *time.Location, override *float64) (kg float64, source string, err error)` implementing the 4-tier rule (explicit > rolling 7d > last-before-date > ErrWeightDataMissing). Move the constants `BodyWeightSourceExplicit` / `BodyWeightSourceRolling7dAvg` / `BodyWeightSourceLastBeforeDate` here and re-export from `summary` so existing callers don't break. Sentinel error `ErrWeightDataMissing` lives here too (re-exported by summary for API stability).
- [x] 1.2 Update `internal/summary/protein.go` to call `bodyweight.ResolveAtDate(...)` instead of the inlined `resolveBodyWeightAtDate`. Delete the inlined function. Keep the package-level `summary.ErrWeightDataMissing` alias pointing at `bodyweight.ErrWeightDataMissing` so existing handler code still compiles.
- [x] 1.3 `internal/bodyweight/resolve_test.go` — table-driven over the four tiers + invalid-override rejection. The protein tests already exercise the cross-package wiring end-to-end; this just nails down the helper's behaviour in isolation.

## 2. raceprep package extension — types + service

- [x] 2.1 `internal/raceprep/recommend.go`: response types
  - `FuelRecommendation{Inputs RecommendInputs; PreWorkout PreWorkout; IntraWorkout IntraWorkout; PostWorkout PostWorkout; Notes []string}`.
  - `RecommendInputs{Sport string; DurationMin int; IntensityZone int; BodyWeightKg float64; BodyWeightSource string; WorkoutID *uuid.UUID ",omitempty"}`.
  - `PreWorkout{WindowMinutesBefore [2]int; CarbsG float64; CarbsGPerKg float64; Rationale string}`.
  - `IntraWorkout{Applicable bool; CarbsGPerHour *float64; CarbsGTotal *float64; FluidMlPerHour *float64; SodiumMgPerHour *float64; Rationale string}` — pointer numerics so `null` round-trips when `Applicable: false`.
  - `PostWorkout{WindowMinutesAfter [2]int; CarbsG float64; ProteinG float64; Rationale string}`.
- [x] 2.2 `RecommendParams{WorkoutID *uuid.UUID; Sport *string; DurationMin *int; IntensityZone *int; BodyWeightKgOverride *float64; Today time.Time; Loc *time.Location}` — validated by the handler; `Today` + `Loc` are needed so the body-weight resolver has a reference date.
- [x] 2.3 Add `Service.RecommendFor(ctx, params) (*FuelRecommendation, error)` next to the existing `CarbLoadFor` / `CarbLoadApply`. Service needs `workouts.Repo` + `bodyweight.Repo` access — extend `NewService` to take both (existing call site in `httpserver/server.go` gets the two new args).

## 3. Mode resolution + workout-row pull

- [x] 3.1 If `params.WorkoutID` is set: load the workout, derive `Sport`, `DurationMin = int(ended_at.Sub(started_at).Minutes())`, `IntensityZone` from §4. 404 `workout_not_found` if the row is missing.
- [x] 3.2 If `params.WorkoutID` is nil: require `Sport` + `DurationMin` + `IntensityZone` to all be non-nil. Validation errors (`sport_required` / `duration_min_required` / `intensity_zone_required`) are returned in first-missing-wins order so the agent has a single field to fix at a time.
- [x] 3.3 Validate `Sport` against the workouts enum (`bike` / `run` / `swim` / `row` / `strength` / `other`) — re-use `workouts.ParseSport`. Validate `IntensityZone ∈ [1, 5]`. Validate `DurationMin > 0` and finite.
- [x] 3.4 Resolve body weight via `bodyweight.ResolveAtDate(ctx, repo, params.Today, params.Loc, params.BodyWeightKgOverride)`.

## 4. Intensity derivation from TSS

- [x] 4.1 `internal/raceprep/intensity.go`: pure helper `IntensityFromTSS(tss *float64, durationMin int) (zone int, defaulted bool)`. Formula: `IF = sqrt(tss / (durationMin / 60) / 100)`; mapping per design table. Returns `(2, true)` when `tss == nil` or `durationMin <= 0`.
- [x] 4.2 Unit tests for the boundary values: `IF = 0.65`, `0.75`, `0.85`, `0.92` — closed-low intervals matching the design.

## 5. Pre-workout recommendation

- [x] 5.1 `internal/raceprep/pre_workout.go`: pure helper `preWorkoutFor(sport string, zone int, bodyWeightKg float64) PreWorkout`. Lookup table:
  ```
  strength      → 0.5 g/kg, window [30, 90]
  zone 5        → 1.0 g/kg, window [60, 90]
  zone 4        → 2.0 g/kg, window [60, 180]
  zone 3        → 1.5 g/kg, window [60, 120]
  zone 1-2      → 1.0 g/kg, window [60, 120]
  ```
- [x] 5.2 Rationale string: name the bucket (`"Zone 3 (tempo) on the bike — 1.5 g/kg in the 1–2h pre-window tops off glycogen for a sustained sub-threshold effort."`).
- [x] 5.3 Round `CarbsG` and `CarbsGPerKg` to 1dp via `numfmt.Round1`.

## 6. Intra-workout recommendation

- [x] 6.1 `internal/raceprep/intra_workout.go`: pure helper `intraWorkoutFor(sport string, durationMin int, zone int) IntraWorkout`. Decision tree per the spec § Intra requirement.
- [x] 6.2 Run-specific cap: when `sport == "run"` AND the duration bucket would yield 90 g/hr, cap at 60 g/hr. Mention the cap explicitly in the rationale.
- [x] 6.3 Strength + (swim AND duration ≤ 120) → `Applicable: false`, all numeric pointers nil, rationale explains why ("Strength sessions get fuelled before and after, not during" / "Swim segments under 2 h rarely allow practical in-session intake").
- [x] 6.4 When `Applicable: true`: compute `CarbsGTotal = round1(perHour × durationMin / 60)`.

## 7. Post-workout recommendation

- [x] 7.1 `internal/raceprep/post_workout.go`: pure helper `postWorkoutFor(bodyWeightKg float64) PostWorkout`. CHO = `1.0 × bodyWeightKg`; Protein = `0.3 × bodyWeightKg` (the MPS threshold from add-protein-distribution). Window `[0, 60]`. Rationale calls out the glycogen-replenishment window + the MPS connection.
- [x] 7.2 Round CarbsG and ProteinG to 1dp.

## 8. Notes builder

- [x] 8.1 `internal/raceprep/recommend_notes.go`: assembles the `notes[]` array. Always include:
  - "Intra-session sodium target is a midpoint; the validated range is 300–800 mg/hr. Heavy sweaters and hot conditions push toward the upper end."
  - "CHO/hr buckets: < 45 min none required; 45–90 min 30 g/hr; 90–180 min 60 g/hr (single transportable, e.g. glucose); > 180 min 90 g/hr (multiple transportable — glucose+fructose 2:1)."
  - "For races > 90 min, also run `plan_carb_load` for the 24–72h pre-loading schedule."
- [x] 8.2 Conditional notes: when intensity was defaulted (TSS absent), append `"Intensity defaulted to Z2 because the workout has no TSS. Pass an explicit intensity_zone if you have RPE/HR context to set it."`. When `sport == "run"` AND the bucket would have produced 90 g/hr, append a note about the run-specific cap.

## 9. HTTP handler

- [x] 9.1 `internal/raceprep/handlers.go`: add `rg.GET("/race-prep/recommend-workout-fuel", h.recommendWorkoutFuel)` next to the existing carb-load registrations.
- [x] 9.2 Parse query params: `workout_id` (optional uuid), `sport` (optional), `duration_min` (optional int), `intensity_zone` (optional int), `body_weight_kg` (optional float). Validate mode exclusivity first (`input_required` / `input_conflict`), then per-field shape.
- [x] 9.3 Materialize `RecommendParams{Today: time.Now(), Loc: time.UTC}` — see design § 7. Call `svc.RecommendFor`. Map service errors:
  - `workouts.ErrNotFound` → `404 workout_not_found`
  - `bodyweight.ErrWeightDataMissing` → `400 weight_data_missing`
  - `ErrBodyWeightInvalid` → `400 body_weight_kg_invalid`
  - other → `500 recommend_failed`
- [x] 9.4 Swag annotations: list every documented error code; reference `FuelRecommendation` as the success type.

## 10. Wiring

- [x] 10.1 `internal/httpserver/server.go`: pass `workoutsRepo` + `bodyWeightRepo` to `raceprep.NewService(...)` (the new args added in §2.3). Existing call site updates to the new constructor signature.

## 11. Backend tests

- [x] 11.1 `internal/raceprep/recommend_test.go` using `storetest.NewPool`:
  - **Explicit-mode**: every bucket → expected pre/intra/post numbers. Table-driven across (sport, duration_min, intensity_zone) → (carbs_g_per_hour, fluid_ml_per_hour, sodium_mg_per_hour). Verify the run cap and the strength/swim not-applicable cases.
  - **Workout-mode**: seed a workout row with `tss` set → derived intensity matches. Seed one with `tss` nil → defaults to Z2 with the disclosure note present.
  - **Workout-mode 404**: unknown id.
  - **Body-weight resolution** smoke-test: stored 72 → 21.6 g protein; explicit override 80 → 24 g protein.
  - **Mode-exclusivity errors**: both modes → `input_conflict`; neither → `input_required`.
  - **Partial-explicit-mode errors**: each of `sport_required` / `duration_min_required` / `intensity_zone_required` reachable.
  - **Invalid values**: `sport=elliptical` → `sport_invalid`; `intensity_zone=0` → `intensity_zone_invalid` (with `range` payload); `duration_min=0` → `duration_min_invalid`; `body_weight_kg=0` → `body_weight_kg_invalid`.
  - **Weight-data-missing**: no entries, no override → 400.
  - **Notes always present**: response always carries the sodium + plan_carb_load notes; TSS-defaulted requests carry the disclosure note.
  - **Rounding**: body weight 72.5 → post protein 21.8 (`0.3 × 72.5 = 21.75` rounds half-away-from-zero to 21.8).
- [x] 11.2 `internal/raceprep/intensity_test.go` — pure-function boundary tests around the IF mapping.

## 12. MCP wrapper

- [x] 12.1 `internal/mcpserver/tools_raceprep.go`: add `RecommendWorkoutFuelArgs{WorkoutID *string; Sport *string; DurationMin *int; IntensityZone *int; BodyWeightKg *float64}` — all pointers so the agent can encode either mode.
- [x] 12.2 `handleRecommendWorkoutFuel`: build the query, omitting any nil field. Call `c.Get(ctx, "/race-prep/recommend-workout-fuel", q)`. No `Idempotency-Key`. Forward via `toToolResult`.
- [x] 12.3 Add the `mcp.AddTool` registration inside the existing race-prep tools registration function. Tool description per the spec — two modes, literature ratios, MPS-threshold reuse, pointer at `plan_carb_load` for loading + `log_workout_fuel` for committing.

## 13. MCP tests

- [x] 13.1 `internal/mcpserver/tools_raceprep_test.go` extension using the recorder pattern:
  - Workout-mode call → only `workout_id` in the query string.
  - Explicit-mode call → only `sport` / `duration_min` / `intensity_zone` (and optional `body_weight_kg`).
  - `400 input_conflict` forwarded as `isError`.
  - `200 OK` response body forwarded byte-for-byte.
- [x] 13.2 `internal/mcpserver/mcp_integration_test.go` expected-tools list: add `recommend_workout_fuel`. Tool count grows by 1.

## 14. Documentation

- [x] 14.1 `task swag` regenerates OpenAPI with the new route + response shape.
- [x] 14.2 `README.md`:
  - "Race prep" subsection gains the recommend example. Show: workout-mode call (90-min Z3 bike via id), explicit-mode call (planned tomorrow ride), and a run example to make the GI-cap behaviour visible.
  - Add `recommend_workout_fuel` to the MCP tools table.
- [x] 14.3 `RUN_LOCAL.md`: append a one-liner showing the explicit-mode call with `body_weight_kg=72` so the example is runnable without seeding weight data first.

## 15. Pre-merge checks

- [x] 15.1 `task vet` clean.
- [x] 15.2 `task test` green per-package — `internal/raceprep/` (intensity + recommend), `internal/mcpserver/` (4 recorder cases + integration tools-list +1), `internal/bodyweight/` (resolver hoist), `internal/summary/` (verifies protein-distribution refactor that now calls `bodyweight.ResolveAtDate`). Multi-package run flaked on testcontainers pool ping in `internal/summary/` — same pattern observed under prior changes; re-running summary in isolation passes.
- [x] 15.3 Manual e2e with `task dev`:
  - Log a body weight.
  - Log a workout with `tss`.
  - `GET /race-prep/recommend-workout-fuel?workout_id=<id>` → assert intensity derived from TSS, post-protein matches the per-meal MPS threshold the protein-distribution endpoint reports for the same body weight.
  - Re-call in explicit-mode for a planned-tomorrow session.
- [x] 15.4 OpenSpec validation: `openspec status --change "add-recommend-workout-fuel"` shows 4/4 artifacts done.
