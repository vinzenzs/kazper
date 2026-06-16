## 1. Shared ±1-day matcher (forward)

- [x] 1.1 Generalize the candidate matcher in `internal/workouts/` to a ±1-day local-day window with same-day preference: query rows in `[date-1, date+1]` for the exact sport; if any candidate is same-day, consider only same-day candidates, else the adjacent-day ones.
- [x] 1.2 Point the forward path (`FindOpenPlanned` used by `reconcileUpsert`) at the shared matcher; the exactly-one / zero / many branches are unchanged.
- [x] 1.3 Unit/integration test the forward tolerance: a cross-day-by-one activity fulfills the only adjacent-day planned; a same-day candidate is preferred over an adjacent-day one (not ambiguous); >1 same-day still flags `needs_link`.

## 2. Reverse reconciliation at materialize

- [x] 2.1 Add `FindAdoptableCompleted(ctx, sport, plannedDate, loc)` to the workouts repo: completed rows with `plan_slot_id IS NULL`, `external_id IS NOT NULL`, exact sport, within the ±1-day window (same-day preferred), reusing the shared matcher.
- [x] 2.2 Add an `Adopt(ctx, completedID, slotInput)` repo op: set `plan_slot_id` + `template_id` from the slot, clear `needs_link`, leave `status='completed'` and actuals intact.
- [x] 2.3 Add `ReconcileSlotOrUpsertPlanned(ctx, tx, in PlannedSlotInput, loc)` to the workouts service/repo: if a row already exists for `in.PlanSlotID` → existing `UpsertPlannedFromSlot` (status-guarded); else match adoptable completed → adopt one / create planned / decline-and-create on >1. Runs inside the caller's transaction.
- [x] 2.4 Point `trainingplan.Materialize` at `ReconcileSlotOrUpsertPlanned` instead of `UpsertPlannedFromSlot`; thread the service timezone (`loc`). Wire any new dependency in `internal/httpserver/server.go` if needed.

## 3. Tests

- [x] 3.1 Integration: import a completed `garmin` activity (no plan) → standalone; then materialize a matching slot → the activity is adopted (gains `plan_slot_id`/`template_id`, `needs_link` cleared, stays completed), no duplicate planned row.
- [x] 3.2 Integration: reverse same-day vs ±1-day (adjacent-day adoption when no same-day; same-day preferred); reverse declines on >1 candidate (planned row created, completed rows untouched).
- [x] 3.3 Integration: re-materialize after adoption is idempotent (no duplicate; completed row unchanged); a `multisport` slot always materializes a planned row (no adoption).
- [x] 3.4 Confirm the forward-path tests in `internal/workouts/reconcile_test.go` still pass with the widened matcher (exact-day cases unchanged).

## 4. Docs & verification

- [x] 4.1 Run `task swag` (behavior-only change; regenerate if any annotation/struct shifted).
- [x] 4.2 Run `task test` and `task vet`; confirm the MCP integration expected-tools list still passes (no new tools).
