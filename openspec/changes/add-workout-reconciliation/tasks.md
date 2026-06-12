# Tasks: add-workout-reconciliation

## 1. Migration (optional, per design D4)

- [ ] 1.1 Verify migration head, then `task migrate:new NAME=add_workout_needs_link`; up: `ALTER workouts ADD needs_link BOOLEAN NOT NULL DEFAULT false`; down: drop it
- [ ] 1.2 `internal/workouts/types.go`: add `NeedsLink bool` (json `needs_link`, omitempty)

## 2. Reconciliation on ingestion

- [ ] 2.1 `internal/workouts/repo.go`: a candidate query — open planned workouts (`status='planned' AND external_id IS NULL`) matching a given sport and local calendar day (date compared `AT TIME ZONE <local>`, reusing the existing local-tz config)
- [ ] 2.2 `internal/workouts/service.go`: in the create + bulk paths, before insert, when `source='garmin'` and the `external_id` is not already stored, run the candidate query: exactly one → merge (set external_id/source/actuals, `status='completed'`, keep template_id/plan_slot_id); zero → insert standalone (unchanged); ≥2 → insert standalone + `needs_link=true`
- [ ] 2.3 Confirm the existing `external_id` UPSERT branch is taken first when the activity is already stored (re-sync idempotency); reconciliation runs only on first sight

## 3. Fulfill / unfulfill

- [ ] 3.1 `service.go`: `Fulfill(plannedId, completedId)` — copy external_id/source/actuals from the completed row onto the planned row, flip to completed, delete the standalone completed row, clear `needs_link`; sentinel errors (not planned / not completed / sport or identity mismatch). `Unfulfill(id)` — clear external_id/source/actuals, restore `status='planned'`, keep template_id/plan_slot_id
- [ ] 3.2 `handlers.go`: `POST /workouts/{plannedId}/fulfill`, `POST /workouts/{id}/unfulfill`; swag annotations; wire in `internal/httpserver`
- [ ] 3.3 Tests: fulfill merges + removes the redundant row + keeps plan_slot_id; unfulfill restores planned + keeps links

## 4. MCP tools

- [ ] 4.1 `internal/mcpserver`: `fulfill_workout`, `unfulfill_workout`; one HTTP call each, verbatim; auto-derive idempotency key
- [ ] 4.2 Bump expected-tools list in `mcp_integration_test.go`

## 5. Integration tests

- [ ] 5.1 Ingestion reconciliation (testcontainers): materialize a planned run, ingest a matching garmin activity → one fulfilled row (status completed, external_id set, plan_slot_id kept); ingest with no plan → standalone; two planned same day/sport → standalone + needs_link; re-sync the fulfilled activity → idempotent, no re-match
- [ ] 5.2 Cross-check the `add-training-plan` guard: after a fulfill, re-materialize the plan and assert the completed row is not reverted

## 6. Docs + verification

- [ ] 6.1 `task swag`; README REST table gains fulfill/unfulfill; README MCP table gains the two tools; a note on the reconcile-on-sync behavior
- [ ] 6.2 `task vet` + `task test` green; `openspec validate add-workout-reconciliation --strict` passes
