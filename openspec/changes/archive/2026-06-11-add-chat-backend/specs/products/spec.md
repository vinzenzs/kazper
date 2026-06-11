# products — delta for add-chat-backend

## ADDED Requirements

### Requirement: Products support a partial update of editable fields

The system SHALL expose `PATCH /products/{id}` accepting any of `{name?, serving_size_g?, nutriments_per_100g?}`. Only supplied fields SHALL change; omitted fields retain their current values, and within `nutriments_per_100g` each supplied nutrient key SHALL be merged individually (setting `kcal` alone MUST NOT clear the others). The endpoint MUST NOT modify `source`, `external_url`, `barcode`, or the last-logged tracking columns. It SHALL return `404 product_not_found` for an unknown id, `400 name_required` for an all-whitespace name, and `400 nutriments_invalid` for a negative nutrient value. The primary caller is the chat agent setting a recipe's nutriments after a serving-size-less Cookidoo import.

#### Scenario: Setting nutriments and serving size on an imported recipe

- **WHEN** a flat-imported recipe product exists with no nutriments
- **AND** the client PATCHes `{serving_size_g: 450, nutriments_per_100g: {kcal: 131, carbs_g: 10}}`
- **THEN** the response is `200` with `serving_size_g = 450`, `kcal = 131`, `carbs_g = 10`
- **AND** the product's `name` and `source` are unchanged
- **AND** nutrients not supplied (e.g. `fat_g`) remain unset

#### Scenario: Nutrient merge does not clear prior fields

- **WHEN** a product already has `kcal = 131` and the client PATCHes `{nutriments_per_100g: {protein_g: 7}}`
- **THEN** the response carries both `protein_g = 7` and the retained `kcal = 131`

#### Scenario: Unknown product is 404

- **WHEN** the client PATCHes a non-existent product id
- **THEN** the response is `404 product_not_found`

#### Scenario: Blank name is rejected

- **WHEN** the client PATCHes `{name: "   "}`
- **THEN** the response is `400 name_required` and the product is unchanged
