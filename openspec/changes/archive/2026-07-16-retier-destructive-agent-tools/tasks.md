## 1. Re-tier the destructive and prescriptive tools

- [x] 1.1 `internal/agenttools/registry_trainingplan.go`: `delete_training_plan`, `delete_plan_week`, `delete_plan_slot`, `add_plan_slot`, `patch_plan_slot`, `materialize_training_plan` → `TierWriteConfirm`
- [x] 1.2 `internal/agenttools/registry_macrocycle.go`: `delete_macrocycle` → `TierWriteConfirm`
- [x] 1.3 `internal/agenttools/registry_races.go`: `delete_race` → `TierWriteConfirm`
- [x] 1.4 `internal/agenttools/registry_workouttemplates.go`: `create_workout_template`, `patch_workout_template`, `delete_workout_template` → `TierWriteConfirm`
- [x] 1.5 `internal/agenttools/registry_trainingphases.go`: `delete_phase`, `set_goal_template`, `delete_goal_template` → `TierWriteConfirm`
- [x] 1.6 Cascade/twin deletes discovered during apply (see design D5): `delete_workout` (`registry_workouts.go`), `delete_product` (`registry_products.go`), `delete_multisport_template` (`registry_multisport.go`), `delete_daily_goal_override` (`registry_goaloverrides.go`) → `TierWriteConfirm`

## 2. Correct the stated policy

- [x] 2.1 `internal/agenttools/registry.go`: extend the Tier doc block — tier is consulted only by the chat loop, the MCP server ignores it, and MCP-only tools carry the tier they would enforce if chat-exposed; state the tiering rule (destructive-of-aggregates or prescription-defining ⇒ confirm)

## 3. Regression guard

- [x] 3.1 Add a registry test asserting every registered tool named `delete_*` carries `TierWriteConfirm` unless it appears in the explicit `cheapDeletes` allowlist (the 10 no-cascade single-row deletes named in the design), and that the explicit prescriptive roster (`add_plan_slot`, `patch_plan_slot`, `materialize_training_plan`, `create_workout_template`, `patch_workout_template`, `set_goal_template`) carries `TierWriteConfirm` too

## 4. Verification

- [x] 4.1 `task vet` and `go test -count=1 ./internal/agenttools/... ./internal/chat/... ./internal/mcpserver/...` — confirm no chat-surface or MCP-surface test regresses (no tool is added or removed, only tiers change)
- [x] 4.2 Confirm zero runtime diff on the exposed surfaces: chat registry tool list unchanged, MCP announced surface unchanged
