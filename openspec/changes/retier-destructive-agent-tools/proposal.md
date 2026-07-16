## Why

The shared registry's tier metadata contradicts its own stated policy. `registry.go` documents `TierWriteConfirm` as "training/goal/destructive writes; pause for human", and the coach-chat spec requires "all delete endpoints SHALL be write-confirm" — yet `delete_training_plan`, `delete_race`, `delete_macrocycle`, `delete_workout_template`, the plan-slot writers, and every other MCP-domain write carries `TierWriteAuto`. Nothing is live-broken today: those tools are MCP-only (`ChatExposed` is never set by any MCP domain, so the chat coach cannot call them) and the MCP server ignores tiers by design. But the tier field is exactly the metadata that becomes load-bearing the moment a domain is chat-exposed — the porting direction this registry exists for — and today it is a landmine: expose the training-plan domain tomorrow and the coach silently gains an unconfirmed `delete_training_plan`. Fixing it now costs zero friction (no runtime behavior changes) and makes the stated policy true.

## What Changes

- **Re-tier to `TierWriteConfirm`** (latent — no behavior change while these stay MCP-only) the tools where a mistake is expensive, irreversible, or prescriptive:
  - Destructive of aggregate/prescriptive state: `delete_training_plan`, `delete_plan_week`, `delete_plan_slot`, `delete_macrocycle`, `delete_race`, `delete_workout_template`, `delete_multisport_template`, `delete_phase`, `delete_goal_template`
  - Deletes that cascade to expensive state, or that mirror an already-confirm chat twin (discovered during apply — see design): `delete_workout` (cascades stored 1 Hz streams), `delete_product` (cascades recipe components), `delete_daily_goal_override` (chat twin is confirm)
  - Prescription writers (what the athlete is told to do): `add_plan_slot`, `patch_plan_slot`, `materialize_training_plan`, `create_workout_template`, `patch_workout_template`, `set_goal_template`
- **Cheap deletes stay `write-auto` via a documented allowlist**: single-row logging, Garmin snapshots, and planner rows that cascade nothing and are cheap to redo (`delete_meal`, `delete_hydration`, `delete_hydration_balance`, `delete_weight`, `delete_workout_fuel`, `delete_planned_meal`, `delete_shopping_item`, `delete_fitness_metrics`, `delete_recovery_metrics`, `delete_coach_memory`). Everything else non-destructive also stays auto: logging writes, entity metadata (`create_training_plan`, `create/update_macrocycle`, `create/update_race`, `create/update_phase`), and all nutrition-planning writes. No blanket re-tier.
- **Correct the doc comments** so stated policy matches code: `registry.go`'s Tier block gains the exposure caveat (tier is consulted only by the chat loop; MCP-only tools carry truthful *latent* tiers), and the tiering rule (destructive + prescriptive ⇒ confirm; cheap no-cascade deletes allowlisted) is written down where the tiers are assigned.
- **Guard against regression**: a registry test asserting every delete-named tool carries `TierWriteConfirm` unless named in the explicit cheap-delete allowlist, and that the prescriptive roster does too — so a future domain port can't silently reintroduce the drift; exempting a delete is a visible, reviewable act.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `coach-chat`: 1 MODIFIED requirement — the tiering requirement extends from "the exposed chat surface is tiered correctly" to "the shared registry's tiers are truthful for every tool regardless of exposure", with the destructive/prescriptive roster pinned by scenario.

## Impact

- **Code:** tier fields in `internal/agenttools/registry_trainingplan.go`, `registry_macrocycle.go`, `registry_races.go`, `registry_workouttemplates.go`, `registry_trainingphases.go`; doc comments in `registry.go`; one new registry test.
- **Runtime:** none today — MCP ignores tiers, chat never sees these tools. The change is insurance that pays out at chat-exposure time.
- **API/docs:** no REST surface change, no `task swag` needed, no migration.
- **Out of scope:** chat-exposing any MCP domain (separate decision, separate change); demoting any of the 12 current write-confirm chat tools (e.g. `log_meal_freeform`) to auto — that is the real friction-vs-safety call and deserves its own conversation with usage evidence; `Format` preview functions for the re-tiered tools (only needed when actually chat-exposed, and the confirmation card path already falls back).
