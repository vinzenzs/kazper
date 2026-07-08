# Tasks — extend plan-adherence with missed list + weekly trend

## 1. Repo: widen the adherence projection

- [x] 1.1 Add `w.id` to the `AdherenceCandidates` SELECT and to `AdherenceRow` (`ID uuid.UUID`).
- [x] 1.2 In plan-scoped mode, add `pw.ordinal` and the week's phase name to the projection — LEFT JOIN `training_phases` on `pw.phase_id` — carried on `AdherenceRow` as nullable fields (populated only when `plan_id` is set). Also load `training_plans.start_date` for that plan (for deriving `week_start`) — via the existing join or a small lookup.
- [x] 1.3 Update the repo integration test for the wider projection: `id` present on every row; plan-scoped rows carry `ordinal`/phase/start_date, unscoped rows do not.

## 2. Service: per-bucket aggregation + missed list

- [x] 2.1 Add response types in `internal/workouts/`: `MissedSession { ID, Date, Sport, PlannedDurationMin, PlannedTSS }`, `WeeklyBucket { WeekStart, Ordinal *int, Phase *string, Completed, Missed int, AdherenceRate, PlannedDurationMin, CompletedDurationMin *float64 }`; extend `AdherenceSummary` with `MissedSessions []MissedSession`, `MissedSessionsTruncated bool`, `Weekly []WeeklyBucket`.
- [x] 2.2 Add a `missedSessionsCap` constant (50).
- [x] 2.3 Refactor `computeAdherence(rows, now, loc)` to fold each classified row into both the window total (unchanged) and a per-bucket accumulator. Bucket key: plan-week `ordinal` when present, else Monday-of-week date of `started_at` in `loc`. Emit buckets only for weeks with ≥1 row; derive `week_start` (plan mode: `start_date + (ordinal-1)*7`; calendar mode: the Monday). Sort `weekly` by `week_start`.
- [x] 2.4 Collect missed sessions in the same pass (append on the missed branch), sort by date ascending, set `MissedSessionsTruncated` when the pre-cap count exceeds the cap, then cap the slice.
- [x] 2.5 Unit-test `computeAdherence`: missed list shape + order; only-missed inclusion; truncation flag at the boundary (exactly cap → false, cap+1 → true); calendar-week bucketing (Monday-start); plan-week bucketing by ordinal with phase + derived `week_start`; per-week null rate for a future-only week; trend totals reconcile with the top-line counts.

## 3. HTTP handler

- [x] 3.1 Thread the plan `start_date` / phase data through `Service.Adherence` so the handler response carries the new fields; no new query params. Keep swag annotations current (new response fields on the existing `GET /workouts/adherence`).
- [x] 3.2 Integration-test the endpoint: unscoped window returns calendar-week `weekly` + `missed_sessions`; `plan_id` window returns plan-week buckets with `ordinal`/`phase`; truncation flag over a large missed set; existing counts/rate unchanged.

## 4. MCP

- [x] 4.1 Confirm `workout_adherence` forwards the richer body verbatim — no new tool, no new args. Verify the announced-schema golden is unchanged (response body is not part of the tool schema); no `goldengen` bump expected. (Golden test green; input schema unchanged.)

## 5. Docs & verification

- [x] 5.1 Run `task swag` to regenerate `docs/` for the grown response shape. (`missed_sessions`/`missed_sessions_truncated`/`weekly` + `MissedSession`/`WeeklyBucket` present in `docs/swagger.json`.)
- [x] 5.2 Run `go test -count=1 ./internal/workouts/...`, the MCP integration test, and `task vet`; note any testcontainers parallel-boot flakes and re-run isolated. (`workouts` ok 198s; MCP announced-schema golden green; `task vet` clean. Pre-existing, unrelated E2E failure: `create_workout_template → not_found`, reproduced on a stashed clean tree — not caused by this change.)
- [x] 5.3 Update `openspec/specs/workouts/spec.md` at archive time (two ADDED requirements synced in).
