## 1. Repo: windowed adherence candidates

- [x] 1.1 Add a repo read returning the in-window candidate workouts for adherence — `status`, `plan_slot_id`, `sport`, `started_at`/`ended_at`, `tss` — over `[from, to]`, with an optional `plan_id` that joins `workouts.plan_slot_id → plan_slots → plan_weeks.plan_id` (excludes no-slot rows when set).
- [x] 1.2 Integration-test the repo read: window bounds inclusive of local dates; plan-scoped query excludes off-plan rows; unscoped includes them.

## 2. Service: classification + aggregation

- [x] 2.1 Add response types (`AdherenceSummary`, `BySportCount`) in `internal/workouts/`: the four counts, `adherence_rate *float64`, `planned_duration_min`/`completed_duration_min`, `planned_tss`/`completed_tss` (`*float64`, null when no present values), `by_sport`.
- [x] 2.2 Add a pure `computeAdherence(rows, now) AdherenceSummary`: classify each row (completed/missed/upcoming/unplanned per the spec), sum planned (completed+missed) vs actual (completed) duration & tss, build by_sport, and `adherence_rate = completed/(completed+missed)` (nil when 0). Round at the boundary via `numfmt`.
- [x] 2.3 Service method `Adherence(ctx, from, to, planID *uuid.UUID) (*AdherenceSummary, error)` that loads candidates and calls `computeAdherence` with `time.Now()` in `s.loc`.
- [x] 2.4 Unit-test `computeAdherence`: the rate (2/3), upcoming excluded, unplanned excluded, null-rate when nothing due, by_sport, planned-vs-actual duration/tss, null tss when none present.

## 3. HTTP handler

- [x] 3.1 Add `GET /workouts/adherence` handler with swag annotations: parse `from`/`to`/`tz` (default zone), optional `plan_id`; validate dates; return the summary. Register in the workouts handler group.
- [x] 3.2 Integration-test the endpoint: a window with completed+missed+upcoming+unplanned returns the right counts/rate; `plan_id` scoping; auth required; invalid date/tz → 400.

## 4. MCP tool

- [x] 4.1 Add `workout_adherence` to `internal/agenttools/registry_workouts.go` — one `GET /workouts/adherence`, args `from`/`to`/`tz`/`plan_id`, read tier.
- [x] 4.2 Bump the `mcp_integration_test.go` expected-tools list (+1) and regenerate the announced-schema golden (`goldengen`).

## 5. Docs & verification

- [x] 5.1 Run `task swag` to regenerate `docs/` for the new endpoint + response shape.
- [x] 5.2 Run `task test` and `task vet`; confirm the MCP integration test passes with the new tool. (`workouts` ok 209s; `vet` clean; integration-tagged MCP test green. The only full-suite FAILs were `energy`/`trainingplan` testcontainers parallel-boot flakes — both pass on isolated `-p 1` re-run.)
