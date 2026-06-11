# Proposal: add-shopping-list

## Why

Selecting dishes for the next few days is only half the loop — the ingredients have to reach the store. Recipes now carry ingredient strings (`add-recipe-ingredients`) and selections are persisted (`add-meal-plan`), but there is nowhere to put "what to buy". Per the project's core principle the API records primitives and the agent does synthesis: the agent merges and dedupes ingredients across the planned days; the API just stores the resulting checklist.

## What Changes

- New `shopping-list` capability: an item is `{name, quantity_text?, recipe_product_id?, plan_date?, checked}` — deliberately a dumb checklist with optional provenance, no unit parsing, no merging logic.
- Bulk create (`POST /shopping/items` accepts an array) so the agent writes a consolidated list in one call.
- List (default unchecked-only), single update (rename, check/uncheck), single delete, and bulk clear of checked items.
- MCP tools mirroring the endpoints.

## Capabilities

### New Capabilities

- `shopping-list`: persistent checklist items with check-off lifecycle and optional links to the recipe and plan date they came from.

### Modified Capabilities

- `mcp-server`: new shopping tools (`add_shopping_items`, `list_shopping_items`, `update_shopping_item`, `delete_shopping_item`, `clear_checked_shopping_items`), each issuing exactly one HTTP call.

## Impact

- **DB**: one migration creating `shopping_items` (optional FK → `products` with ON DELETE SET NULL).
- **Code**: new `internal/shoppinglist` package (standard capability shape, no cross-injection — provenance FKs are best-effort, not validated business links); `internal/httpserver` wiring; `internal/mcpserver` tools + expected-tools bump.
- **Docs**: `task swag`.
- **Sequencing**: independent — no hard dependency on the other four changes (provenance fields are nullable); consumed by `add-chat-backend` and `add-companion-chat`.
