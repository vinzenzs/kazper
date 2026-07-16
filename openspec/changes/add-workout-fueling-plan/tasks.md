# Tasks

## 1. Backend

- [ ] 1.1 `internal/workoutfueling/fuelingplan.go`: pure math (TSS→kJ, IF derivation + CHO ladder, duration ladder + capacity clamp, deficit) + unit tests (worked 3 h/180 TSS fixture, IF band boundaries, short-session zero, clamp, each degradation reason, rounding)
- [ ] 1.2 Handler `GET /workouts/{id}/fueling-plan`: effective-FTP via the wiring-trunk adapter, `workout_not_planned` 409, `carbs_per_hr` validation, layered degradations; **re-base the athlete-config MODIFIED delta against the merged spec at apply** (the gate req is high-churn — verify no sibling language is clobbered)
- [ ] 1.3 Integration tests: planned-ride happy path, capacity clamp, short session, tss/ftp/plan-data degradations, completed-workout 409, read-only, unit isolation (no daily-total leakage)
- [ ] 1.4 `task swag`

## 2. MCP

- [ ] 2.1 `workout_fueling_plan` read tool (race-vs-training division + capacity framing in description); golden regen (additive) + registry/integration green

## 3. Verification

- [ ] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 3.2 Live sanity: run the plan for the next real long ride; eyeball burn vs the watch's kJ after completion and prescription vs what was actually carried
