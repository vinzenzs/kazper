# shopping-list — delta for add-shopping-list

## ADDED Requirements

### Requirement: Shopping items are flat checklist rows with optional provenance

The system SHALL persist a shopping item as `{name, quantity_text?, recipe_product_id?, plan_date?, checked, created_at, checked_at?}`. `name` is required (non-empty, ≤ 300 chars); `quantity_text` is opaque free text the system MUST NOT parse or aggregate; `recipe_product_id`, when supplied, MUST reference an existing product (else `404 product_not_found`) and SHALL be set NULL if that product is later deleted; `plan_date` is a bare date with no FK semantics. Items default to `checked: false`.

#### Scenario: Item with provenance round-trips

- **WHEN** the client creates an item `{name: "Staudensellerie", quantity_text: "100 g", recipe_product_id: <recipe>, plan_date: "2026-06-13"}`
- **THEN** the item is persisted and returned with `checked: false` and all fields intact

#### Scenario: Dangling provenance never blocks the list

- **WHEN** the product referenced by an item's `recipe_product_id` is deleted
- **THEN** the item remains with `recipe_product_id: null` and is still listed and checkable

### Requirement: Bulk create writes a consolidated list atomically

The system SHALL expose `POST /shopping/items` accepting `{items: [...]}` with 1–200 entries, inserting all atomically — if any entry fails validation, no rows are created. The endpoint SHALL participate in the standard idempotency middleware. The response carries the created items in input order.

#### Scenario: Agent writes a merged list in one call

- **WHEN** the client POSTs 14 items consolidated from three days of planned recipes
- **THEN** the response is `201` with 14 items in input order, all `checked: false`

#### Scenario: One invalid entry fails the whole batch

- **WHEN** the client POSTs 5 items where the third has an empty `name`
- **THEN** the response is a `400` validation error identifying the offending index
- **AND** zero rows are created

### Requirement: Listing defaults to open items; checked items are retrievable last

`GET /shopping/items` SHALL return unchecked items ordered by `created_at` ascending. With `?include_checked=true` it SHALL additionally return checked items after all unchecked ones.

#### Scenario: Default list hides bought items

- **WHEN** the list has 3 unchecked and 2 checked items and the client GETs `/shopping/items`
- **THEN** the response contains exactly the 3 unchecked items in creation order

#### Scenario: include_checked appends bought items

- **WHEN** the same client GETs `/shopping/items?include_checked=true`
- **THEN** the response contains all 5 items with the 2 checked ones last

### Requirement: Check-off lifecycle via PATCH and bulk clear of checked items

The system SHALL expose `PATCH /shopping/items/{id}` accepting any of `{checked, name, quantity_text}`; setting `checked: true` records `checked_at` server-side and `checked: false` clears it. `DELETE /shopping/items/{id}` removes a single item. `DELETE /shopping/items?checked=true` SHALL delete all currently checked items and report the count; an unqualified bulk delete MUST NOT exist.

#### Scenario: Checking an item stamps checked_at

- **WHEN** the client PATCHes an item with `{checked: true}`
- **THEN** the response shows `checked: true` with a non-null `checked_at`
- **AND** a later PATCH `{checked: false}` returns `checked_at` to null

#### Scenario: Clear checked reports the count

- **WHEN** 4 items are checked and the client DELETEs `/shopping/items?checked=true`
- **THEN** the response reports 4 deleted and only unchecked items remain

#### Scenario: Unqualified bulk delete is not available

- **WHEN** the client DELETEs `/shopping/items` without the `checked=true` query
- **THEN** the response is a `400` validation error and no items are deleted
