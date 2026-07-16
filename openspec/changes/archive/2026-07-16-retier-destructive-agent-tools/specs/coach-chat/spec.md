## MODIFIED Requirements

### Requirement: The tool surface is the full coach surface, tiered for confirmation

The chat loop SHALL expose a **curated** coaching tool surface (~15–25 tools), not the desktop MCP server's full surface — favoring a few **aggregate context reads** (e.g. `get_daily_context`, `get_training_context`, `get_recovery_context`) over many granular reads, plus the `write-auto` meal-planning writes and the `write-confirm` actions worth proposing conversationally — sourced from a shared tool registry (`internal/agenttools`). A drift-guard test SHALL assert every tool the chat loop exposes exists in the desktop MCP server's announced surface (modulo a documented allowlist of chat-bespoke convenience tools), so the two cannot silently diverge even though the MCP server is not yet ported onto the shared registry. Each tool SHALL carry a `tier`: `read` (never gated), `write-auto` (low-stakes nutrition-planning writes that dispatch inline), or `write-confirm` (training, goal, and destructive writes that pause for human confirmation). The existing planner writes — `import_cookidoo_recipe`, `update_product`, `create_planned_meal`, `update_planned_meal`, `mark_planned_meal_eaten`, `add_shopping_items`, `update_shopping_item`, `clear_checked_shopping_items` — SHALL be `write-auto`. Training/Garmin/goal/override edits and all delete endpoints SHALL be `write-confirm`. Tiers SHALL be **truthful for every registered tool regardless of exposure**: tools registered on the MCP-only surface carry the tier the policy implies even though the MCP server ignores tiers, so that chat-exposing a domain never silently grants unconfirmed destructive writes. Concretely, every tool that deletes prescriptive or aggregate state (`delete_training_plan`, `delete_plan_week`, `delete_plan_slot`, `delete_macrocycle`, `delete_race`, `delete_workout_template`, `delete_multisport_template`, `delete_phase`, `delete_goal_template`), every delete whose row cascades to or permanently destroys expensive state (`delete_workout` — cascades the stored 1 Hz streams; `delete_product` — permanently destroys the catalog record, though recipe use is service-guarded via `409 product_in_use_as_component`) or that mirrors an already-confirm chat twin (`delete_daily_goal_override`), and every prescription writer (`add_plan_slot`, `patch_plan_slot`, `materialize_training_plan`, `create_workout_template`, `patch_workout_template`, `set_goal_template`) SHALL carry `write-confirm`. A registry test SHALL enforce that every tool named `delete_*` carries `write-confirm` **unless it appears in a documented cheap-delete allowlist** — single-row logging, Garmin snapshots, and planner rows that cascade nothing and are cheap to redo — and that every tool in the prescriptive roster does. The allowlist SHALL be explicit in the test, so exempting a delete is a deliberate, reviewable act rather than an omission. The loop SHALL also include Anthropic's `web_search` server tool restricted via `allowed_domains` to Cookidoo hosts.

#### Scenario: Planner writes stay fast

- **WHEN** the user accepts three days of dinners
- **THEN** the `create_planned_meal` and `add_shopping_items` calls dispatch inline without a confirmation pause (they are `write-auto`)

#### Scenario: Coaching reads are aggregate, not granular

- **WHEN** the coach grounds before training advice
- **THEN** it calls a small number of aggregate context reads (e.g. `get_training_context`, `get_recovery_context`) rather than many granular per-metric Garmin tools
- **AND** the exposed surface stays around 15–25 tools, not the desktop server's full 127

#### Scenario: Training and destructive writes are gated

- **WHEN** the agent requests a goal change, a workout schedule, or any delete
- **THEN** that call is `write-confirm` and the turn pauses with a `proposal` rather than dispatching

#### Scenario: Unexposed destructive tools carry the confirm tier anyway

- **WHEN** the registry's MCP-only training-plan domain is inspected
- **THEN** `delete_training_plan`, `delete_plan_week`, `delete_plan_slot`, `add_plan_slot`, `patch_plan_slot`, and `materialize_training_plan` carry `write-confirm` even though no chat surface exposes them and the MCP server ignores the tier

#### Scenario: A future domain port cannot silently reintroduce auto deletes

- **WHEN** a new registry domain adds a tool named `delete_something` with `write-auto`
- **THEN** the registry tier test fails, forcing a deliberate tiering decision — either confirm the tool or add it to the documented cheap-delete allowlist

#### Scenario: Cheap deletes stay auto by name, not by accident

- **WHEN** the registry is inspected for `delete_*` tools carrying `write-auto`
- **THEN** every one of them appears in the test's cheap-delete allowlist (single-row logging, Garmin snapshots, planner rows), and no delete that cascades to expensive state is among them

#### Scenario: Chat sources from the shared registry, guarded against drift

- **WHEN** the chat tool defs are constructed
- **THEN** they derive from the shared `internal/agenttools` registry (name, schema, loopback HTTP mapping, tier)
- **AND** a test asserts every exposed tool name exists in the MCP server's announced surface (modulo the documented chat-bespoke allowlist), failing if a coach tool is added to one surface but not reconciled with the other
