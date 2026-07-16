## Context

The shared `internal/agenttools` registry serves two consumers with different trust models: the in-app chat loop reads `Tier` to decide whether a write pauses for a confirmation card, and the desktop MCP server ignores `Tier` entirely (the MCP client's own permission model is the gate — DD1/DD3 of unify-mcp-tool-registry). Chat sees only `ChatExposed` specs — today the curated planner + coach-read + 12 write-confirm surface. The ~100 MCP-domain tools registered via `registerMCPDomain` set only `MCPExposed`, and all of their writes were stamped `TierWriteAuto` as they were ported, including plan/race/macrocycle/template deletes — contradicting both the `registry.go` Tier doc ("training/goal/destructive writes; pause for human") and the coach-chat spec ("all delete endpoints SHALL be write-confirm").

## Goals / Non-Goals

**Goals:**
- Every registry tool carries the tier the stated policy implies, whether or not anything reads it today — so chat-exposing a domain never silently grants unconfirmed destructive writes.
- The tiering rule is written down once, next to the tiers, and enforced by a test.
- Zero runtime behavior change.

**Non-Goals:**
- Chat-exposing any MCP domain.
- Revisiting the current 12 chat write-confirm tools (the logging-friction question needs usage evidence, not this change).
- Adding `Format` confirmation previews to MCP-only tools.

## Decisions

**1. The tiering rule: destructive-of-aggregates OR prescription-defining ⇒ confirm.**
A tool is `TierWriteConfirm` when a wrong call either destroys state that is expensive to rebuild (a plan, week, slot, macrocycle, race, template, phase, goal template) or changes what the athlete is told to do (slot add/patch, template create/patch, goal-template set, plan materialization). Single-row logging and entity *metadata* creation/update stay auto — a mis-created empty plan or a renamed macrocycle is cheap to notice and cheap to undo.
*Alternative considered:* re-tiering all ~100 MCP writes — rejected (the memo's own reasoning): it would make the coach unusable for routine logging the day any domain is chat-exposed, and blanket rules invite blanket exceptions.

**2. `materialize_training_plan` is confirm, despite being idempotent.**
It is slot-keyed and re-runnable, but it is the single highest-leverage prescription write in the registry — one call rewrites the athlete's dated planned workouts across a whole scope. Idempotency protects against duplication, not against materializing the wrong plan edit. Confirm.

**3. Creates/updates of plan, macrocycle, race, and phase stay auto.**
These are containers and metadata; the prescriptive damage flows through slots, templates, and goal templates, which are all confirm under the rule. `update_race` (date/priority) arguably reshapes a taper, but it is visible, non-destructive, and trivially re-editable — and the line has to hold somewhere or the rule collapses into "everything is confirm."

**4. Latent tiers are documented as latent.**
The `registry.go` Tier doc gains one sentence: only the chat loop consults tiers, the MCP server never does, and MCP-only tools carry the tier they *would* enforce if exposed. This kills the current false-guarantee reading without pretending the tier gates MCP.

**5. Regression guard as a name-pattern + roster test, with a named cheap-delete allowlist.**
A test in `internal/agenttools` asserts (a) every registered tool whose name starts with `delete_` carries `TierWriteConfirm` unless it appears in an explicit `cheapDeletes` allowlist, and (b) the explicit prescriptive roster (add/patch slot, create/patch template, set_goal_template, materialize) does too. The `delete_` pattern makes future domain ports fail closed; the roster pins the judgment calls; the allowlist keeps routine logging cheap without turning the pattern off.

*Corrected during apply — the original claim here was wrong.* This decision previously read "(Existing chat delete tools — `delete_workout`, `delete_daily_goal_override` — already conform, so the pattern rule holds registry-wide.)" That is true only of their **chat** registrations. **21 tool names are registered twice** — once `ChatExposed`, once `MCPExposed`, as separate specs — and 9 carry different tiers on the two (chat `write-confirm`, MCP `write-auto`), including both of those deletes. A registry-wide pattern rule therefore failed on **14** MCP `delete_*` tools, not zero, and the rule as stated collided with the proposal's own "logging stays auto" line.

The allowlist resolves the collision by naming the cheap deletes rather than weakening the rule. Membership is decided by *what the delete destroys*, verified against the schema, not by how the tool sounds:

- **`delete_workout` → confirm** (NOT allowlisted): `workout_streams` is `ON DELETE CASCADE` (migration 036), so deleting a workout destroys its 1 Hz series — recoverable only by a Garmin re-post. Also aligns its MCP tier with its already-confirm chat twin.
- **`delete_product` → confirm**: permanent catalog destruction — meal history survives (`meal_entries.product_id` is `ON DELETE SET NULL` with snapshot columns carrying the nutriments), but the product record itself (nutriments, serving data, barcode) is gone for good and only re-creatable by hand or OFF re-import. *Corrected during apply:* this bullet originally justified confirm via the `recipe_components` `ON DELETE CASCADE` (migration 006) "silently destroying" component lists — that cascade never fires through the API, because the service refuses in-use deletes with `409 product_in_use_as_component` (`internal/products/service.go`). Confirm stands on permanence, not on a cascade; admittedly the most borderline member of the confirm set.
- **`delete_multisport_template` → confirm**: a template is prescriptive state, the same class as `delete_workout_template`, which this change already confirms.
- **`delete_daily_goal_override` → confirm**: aligns its MCP tier with its already-confirm chat twin; a split tier on one tool name is precisely the drift this change exists to remove.

`cheapDeletes` (10) is then only single-row logging, Garmin snapshots, and planner rows — each cheap to notice and cheap to redo, and none with a cascade: `delete_meal`, `delete_hydration`, `delete_hydration_balance`, `delete_weight`, `delete_workout_fuel`, `delete_planned_meal`, `delete_shopping_item`, `delete_fitness_metrics`, `delete_recovery_metrics`, `delete_coach_memory`.

*Alternative considered:* dropping the `delete_` pattern and testing only the roster — rejected; it discards the fail-closed property that is the guard's whole point. *Also considered:* re-tiering all 14 to confirm — rejected for the proposal's stated reason (it would gate routine logging the day a domain is chat-exposed).

## Risks / Trade-offs

- **[9 tool names still carry two different tiers across their chat and MCP registrations]** (`patch_workout`, `log_workout`, `log_weight`, `log_hydration`, `log_meal_freeform`, `set_daily_goal_override`, `remember`, and — until this change — `delete_workout`/`delete_daily_goal_override`) → this change fixes only the two deletes, because they fall under its rule. The remaining 7 are a real self-contradiction in the same metadata this change exists to make truthful, but resolving them means deciding whether logging is confirm-worthy — the friction question this proposal explicitly defers. **Left as a follow-up**, and named here so the next reader finds it rather than rediscovers it.
- **[A future chat exposure inherits confirm friction on slot edits]** → intended; the confirmation card is the commit point for prescriptions. If it proves too chatty, demote deliberately with evidence, not by default.
- **[Re-tiered tools lack `Format` previews]** → the confirmation-card path falls back to a generic preview; previews become worth writing only at exposure time, and the drift-guard conversation happens then anyway.
- **[`delete_` name-pattern test could catch a genuinely trivial future delete]** → acceptable; overriding requires touching the test, which is exactly the deliberate-decision moment the guard exists to force.

## Migration Plan

No migration, no deploy urgency — metadata-only. Ships with the normal release train.

## Open Questions

_None._
