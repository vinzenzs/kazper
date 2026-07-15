## Context

Meals log in two modes: product-referenced (`product_id` + `quantity_g`, nutrients derived per-100 g) and freeform (nutrients supplied directly). The modes are fixed at creation; converting requires delete + re-log, which loses the entry id, `logged_at`, note, and workout link — and breaks the "same meal, better data" intent of a correction.

## Goals / Non-Goals

**Goals:** one dedicated write that swaps a meal's nutrient *source* while preserving its identity and links; derivation identical to product-mode creation so a corrected meal is indistinguishable from one logged right the first time.

**Non-Goals:** an audit trail of prior values, PATCH-style partial nutrient edits (freeform corrections stay delete + re-log), reversing product→freeform, bulk sweeps.

## Decisions

### D1 — A dedicated action endpoint, not PATCH overloading
`POST /meals/{id}/correct-product` — the operation is "replace the nutrient source", an action with invariants (derive-all-from-product), not a field-level patch. Overloading meal PATCH with mode-conversion semantics would tangle its tri-state conventions.

### D2 — Full re-derivation, same code path as product-mode create
All nutrient fields are recomputed from the product per-100 g values × `quantity_g` — the exact function product-mode logging uses, so there is one derivation truth. Prior values are overwritten; the meal's mode becomes product-referenced.

### D3 — Preserved vs replaced, explicitly
Preserved: id, `logged_at`, note, `workout_id` link, `created_at`. Replaced: all nutrient fields, `product_id`, `quantity_g`, description/name follows the product (the entry now *is* that product serving). `updated_at` moves.

### D4 — Idempotency-Key accepted (POST convention); errors 1:1
`product_id_required`, `product_not_found`, `quantity_invalid` (> 0), `not_found` — matching the meals error vocabulary.

## Risks / Trade-offs

- **Silent value overwrite** — deliberate (a correction supersedes a guess); the MCP tool is write-tier so the chat surface confirms before dispatch, and the response returns the corrected entry for immediate visibility.
- **Name/description replacement may surprise** ("pasta at Anna's" → product name) — accepted for v1; the note field survives and carries the context.

## Migration Plan

None (existing columns). Rollback = revert route/tool.

## Open Questions

- Should the pre-correction values be dropped into the note automatically ("was: ~600 kcal est.")? (v1: no — the coach can do it explicitly when it matters.)
