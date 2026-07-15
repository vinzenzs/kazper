## ADDED Requirements

### Requirement: A meal is retroactively correctable to a product

The system SHALL expose `POST /api/v1/meals/{id}/correct-product` accepting
`{product_id, quantity_g}` (standard `Idempotency-Key` supported): the meal's nutrient fields
SHALL be fully re-derived from the referenced product per 100 g × `quantity_g` using the same
derivation as product-mode logging, the product reference and quantity SHALL be set, and the
entry's id, `logged_at`, note, workout link, and `created_at` SHALL be preserved (`updated_at`
moves). The correction SHALL work on freeform and product-referenced meals alike. Errors:
`400 product_id_required`, `404 product_not_found`, `400 quantity_invalid` (missing or ≤ 0),
`404 not_found` for an unknown meal. The response SHALL return the corrected entry; daily and
range summaries SHALL reflect the corrected values with no further action. A
`correct_meal_product` MCP tool (write tier) SHALL wrap the endpoint in one call.

#### Scenario: A freeform guess becomes a product serving

- **WHEN** a freeform meal with estimated macros is corrected with a valid `product_id` and
  `quantity_g: 150`
- **THEN** its nutrients equal the product's per-100 g values × 1.5, the product link and
  quantity are set, and `logged_at`, note, and workout link are unchanged

#### Scenario: A wrong product is re-correctable

- **WHEN** a product-referenced meal is corrected to a different product
- **THEN** nutrients re-derive from the new product and the entry identity is preserved

#### Scenario: The day's summary follows the correction

- **WHEN** a past day's meal is corrected
- **THEN** that day's summary reflects the re-derived values on the next read

#### Scenario: An unknown product is rejected

- **WHEN** the supplied `product_id` does not exist
- **THEN** the response is `404` with `product_not_found` and the meal is unchanged
