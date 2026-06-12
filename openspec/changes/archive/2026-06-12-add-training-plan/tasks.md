# Tasks: add-training-plan

## 1. Migration

- [x] 1.1 Verify migration head is `030` (after add-workout-templates), then `task migrate:new NAME=add_training_plan` (expect `031`)
- [x] 1.2 Up migration: create `training_plans`, `plan_weeks` (UNIQUE `(plan_id, ordinal)`, `phase_id` FK SET NULL), `plan_slots` (`weekday` CHECK 0–6, `template_id` FK RESTRICT, `time_of_day` nullable) per design D1
- [x] 1.3 Same migration: `ALTER workouts ADD template_id` (FK SET NULL) `ADD plan_slot_id` (FK SET NULL); create partial UNIQUE index `workouts_plan_slot_id_key ON (plan_slot_id) WHERE plan_slot_id IS NOT NULL`
- [x] 1.4 Down migration: drop index + two workouts columns, then the three tables in reverse FK order

## 2. workouts capability additions

- [x] 2.1 `internal/workouts/types.go`: add `TemplateID *uuid.UUID` and `PlanSlotID *uuid.UUID` (json omitempty)
- [x] 2.2 `internal/workouts/repo.go`: `UpsertPlannedFromSlot(ctx, q, input)` — `INSERT … ON CONFLICT (plan_slot_id) WHERE plan_slot_id IS NOT NULL DO UPDATE … WHERE workouts.status = 'planned'` setting sport/name/status/template_id/started_at/ended_at/session_group; returns the row. The `status='planned'` guard prevents reverting a fulfilled (completed) workout. Confirm existing `external_id` upsert is untouched
- [x] 2.3 Tests: slot upsert is idempotent; slot path and external_id path coexist without collision; **a completed row sharing the slot is not reverted by re-upsert** (guard holds)

## 3. training-plan package — types + repo

- [x] 3.1 `internal/trainingplan/types.go`: `Plan`, `PlanWeek`, `PlanSlot` structs; nested tree shape for `GET /{id}`
- [x] 3.2 `repo.go`: plan CRUD; week create/patch/delete; slot create/patch/delete; a `Load(id)` that returns the nested tree (plan → weeks → ordered slots) against `store.Querier`

## 4. training-plan service — materialize

- [x] 4.1 `service.go`: validation + sentinel errors (ordinal ≥ 1, weekday 0–6, template exists, plan/week/slot existence, RESTRICT surfaced when a referenced template is deleted)
- [x] 4.2 `Materialize(ctx, planID, scope)` in a `store.WithTx`: resolve in-scope weeks (week N / date range / all), compute `date = start_date + (ordinal-1)*7 + weekday`, derive the time window (`time_of_day` or 06:00 stacked by slot ordinal; end = start + template duration or 1h), call `workouts.UpsertPlannedFromSlot` per slot, assign a shared `session_group` for multi-slot days; return the planned workouts
- [x] 4.3 Unit/integration tests: week materializes to correct dates; re-materialize idempotent; editing a slot retargets; shifting `start_date` moves all; multi-slot day shares a session_group; completed activity on the same date is untouched

## 5. Handlers + wiring

- [x] 5.1 `handlers.go`: plan CRUD + nested GET; week sub-resources; slot sub-resources; `POST /training-plans/{id}/materialize`; swag annotations; `Register(rg)`
- [x] 5.2 Wire in `internal/httpserver/server.go` (repo+service, inject the workouts repo for the slot upsert; routes behind auth + idempotency)

## 6. MCP tools

- [x] 6.1 `internal/mcpserver/server.go`: `registerTrainingPlanTools` — `create_/list_/get_/patch_/delete_training_plan`, `add_/patch_/delete_plan_week`, `add_/patch_/delete_plan_slot`, `materialize_training_plan`; one HTTP call each, verbatim; write tools auto-derive idempotency key
- [x] 6.2 Bump expected-tools list in `mcp_integration_test.go`

## 7. Integration tests

- [x] 7.1 Handler integration tests (testcontainers): build a small plan via the API, materialize a week, assert planned workouts land on the right dates with the template sport/name; re-materialize idempotent; delete plan cascades + nulls `plan_slot_id` on workouts; template RESTRICT on delete

## 8. Docs + verification

- [x] 8.1 `task swag`; README REST table gains plan/week/slot/materialize endpoints; README MCP table gains the twelve tools
- [x] 8.2 `task vet` + `task test` green; `openspec validate add-training-plan --strict` passes
