## Why

The oldest still-open triage item: meals logged freeform ("~600 kcal pasta, guessed macros") can't later be corrected to a real product once it's scanned or created — today that's delete + re-log, losing the entry's identity, timestamps, and links. A retroactive correction keeps the log honest without rewriting history's shape.

## What Changes

- `POST /api/v1/meals/{id}/correct-product` with `{product_id, quantity_g}`: re-derives the meal's nutrient fields from the product (per-100 g × quantity, the same derivation as product-mode logging), sets the product reference and quantity, and **preserves** the entry's identity, `logged_at`, note, and any workout link. Works on freeform meals and on product meals (fixing a wrong product/quantity alike). Accepts the standard `Idempotency-Key`.
- Errors 1:1: `404 not_found` (meal), `404 product_not_found`, `400 quantity_invalid`, `400 product_id_required`.
- New `correct_meal_product` MCP tool (write tier) — the coach flow this exists for: "that pasta from Tuesday is actually this Cookidoo recipe."
- Daily/range summaries reflect the corrected values automatically (they read stored nutrients; no summary change).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `meals`: 1 ADDED requirement — the retroactive product correction.

## Impact

- **Code:** `internal/meals` service method + handler (reuses the existing product-derivation path); MCP registry + golden additive; `task swag`. No migration (uses existing meal columns).
- **Out of scope:** correction history/audit trail (the meal's `updated_at` moves, nothing else is recorded), bulk correction, product→freeform reversal (delete + re-log remains that path).
