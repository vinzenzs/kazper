# Tasks

## 1. Backend

- [x] 1.1 `internal/workoutfueling/fuelingplan.go`: pure math (TSS→kJ, IF derivation + CHO ladder, duration ladder + capacity clamp, deficit) + unit tests (worked 3 h/180 TSS fixture, IF band boundaries, short-session zero, clamp, each degradation reason, rounding)
- [x] 1.2 Handler `GET /workouts/{id}/fueling-plan`: effective-FTP via the wiring-trunk adapter, `workout_not_planned` 409, `carbs_per_hr` validation, layered degradations; **re-base the athlete-config MODIFIED delta against the merged spec at apply**
      _Re-base verified by diffing the delta against the merged requirement: it is a clean superset — every sibling clause (training-plan zones, IF derivation, per-sport TSS, race-pacing) and every existing scenario header is preserved verbatim; the delta adds only the workout-fuel FTP sentence + one new scenario. Nothing clobbered._
- [x] 1.3 Integration tests: planned-ride happy path, capacity clamp, short session, tss/ftp/plan-data degradations, completed-workout 409, read-only, unit isolation (no daily-total leakage)
      _Note: `plan_data_missing` is UNREACHABLE through the endpoint — `workouts` has `CHECK (ended_at > started_at)` (migration 012), so a stored workout always has a duration and `tss_missing` is the reachable floor. The branch is kept as defense-in-depth (covered by the pure-math tests); an integration test pins the schema constraint that makes it unreachable._
- [x] 1.4 `task swag`

## 2. MCP

- [x] 2.1 `workout_fueling_plan` read tool (race-vs-training division + capacity framing in description); golden regen (additive) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 3.2 Live sanity: run the plan for the next real long ride; eyeball burn vs the watch's kJ after completion and prescription vs what was actually carried
      _(operator step — needs a real ride and the watch's post-hoc kJ; not runnable in-session)_
