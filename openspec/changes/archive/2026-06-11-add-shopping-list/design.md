# Design: add-shopping-list

## Context

The chat agent (and desktop agent) will compose shopping lists from planned recipes' ingredient strings — merging "1 Zwiebel" + "2 Zwiebeln" is LLM work, language-mixed and judgment-laden. The store underneath must therefore be the dumbest thing that works: ordered text items with a checked flag. The companion app renders it with check-off; checks flow through the existing offline outbox.

## Goals / Non-Goals

**Goals:**
- Bulk write (one agent tool call → whole consolidated list).
- Check-off that works offline through the app outbox.
- Optional provenance (`recipe_product_id`, `plan_date`) so "why is this on my list?" is answerable.
- Cheap recurring hygiene (clear checked).

**Non-Goals:**
- No quantity/unit parsing or arithmetic — `quantity_text` is opaque ("3 large").
- No server-side merging, deduping, or categorization (aisle grouping etc.) — agent-side.
- No multiple named lists — one global list (single user, one household).

## Decisions

### D1: Flat items, `quantity_text` opaque

`{id, name, quantity_text?, recipe_product_id?, plan_date?, checked, created_at, checked_at?}`. Parsing "100 g" into (100, grams) invites a unit system the project has deliberately avoided merging across capabilities; the only consumer is a human at a shop shelf. Alternative — structured qty/unit columns — rejected as synthesis in the API layer.

### D2: Bulk create is the primary write

`POST /shopping/items` accepts `{items: [...]}` (1–200). The agent's natural output is a whole list; N round trips would multiply idempotency keys and partial-failure states. The bulk call is atomic — all items insert or none. Single-item creation is the same endpoint with a one-element array (no separate endpoint).

### D3: Provenance FK is soft

`recipe_product_id` references `products` with `ON DELETE SET NULL`, and a nonexistent id at create time is a `404 product_not_found` (cheap existence check), but the link carries no other semantics — deleting the plan or product never deletes shopping items. `plan_date` is a bare date column, not an FK to `planned_meals` (items aggregate several plan entries; a row-level FK would be wrong anyway).

### D4: Check-off is a PATCH; clearing is a dedicated bulk delete

`PATCH /shopping/items/{id}` with `{checked: true|false, name?, quantity_text?}` sets/clears `checked_at` server-side. `DELETE /shopping/items?checked=true` clears bought items in one call (the only bulk delete; an unqualified bulk delete is intentionally absent — wiping the whole list should be N explicit deletes or agent judgment). Default `GET` returns unchecked only; `?include_checked=true` returns all, checked last.

### D5: Standard package, no service cross-injection beyond products existence check

`internal/shoppinglist/{types,repo,service,handlers}` per the template, products repo injected for the existence check only — mirrors how other capabilities validate FK presence.

## Risks / Trade-offs

- [Checked items accumulate forever if never cleared] → `checked_at` enables future TTL hygiene; v1 leaves cleanup manual/agent-driven.
- [Concurrent agent writes could duplicate items textually] → accepted; dedup is agent-side by design, and the single-user reality makes write races rare.
- [Soft provenance can dangle (plan deleted, product gone)] → accepted; provenance is a hint for display, never joined for correctness.

## Migration Plan

One migration: `shopping_items` table per D1/D3. Down drops it. Additive deploy, safe rollback.

## Open Questions

- None blocking.
