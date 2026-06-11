# mcp-server — delta for add-shopping-list

## ADDED Requirements

### Requirement: Shopping tools mirror the shopping-list REST endpoints

The MCP server SHALL expose `add_shopping_items` (bulk array), `list_shopping_items` (`{include_checked?}`), `update_shopping_item`, `delete_shopping_item`, and `clear_checked_shopping_items`, each issuing exactly one HTTP call and forwarding the response body verbatim, with idempotency keys auto-derived on writes. The `add_shopping_items` description SHALL instruct the agent to merge and dedupe ingredient quantities across recipes BEFORE calling — the API stores items verbatim and never aggregates.

#### Scenario: Bulk add is one HTTP call

- **WHEN** the agent calls `add_shopping_items` with 14 merged items
- **THEN** the MCP server issues a single `POST /shopping/items` with the full array and an auto-derived idempotency key
- **AND** the tool result is the REST response body verbatim

#### Scenario: Validation failure surfaces as isError

- **WHEN** the batch contains an empty-name item
- **THEN** the tool result has `isError=true` carrying the REST 400 body
