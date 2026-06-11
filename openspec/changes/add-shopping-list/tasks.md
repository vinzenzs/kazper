# Tasks: add-shopping-list

## 1. Storage

- [x] 1.1 Check migration head, then `task migrate:new NAME=add_shopping_items` — `shopping_items` per design (product FK ON DELETE SET NULL, checked default false, checked_at nullable)

## 2. Capability package

- [x] 2.1 `internal/shoppinglist/types.go` — Item struct, JSON tags with `omitempty` nullables
- [x] 2.2 `repo.go` — bulk insert (single statement, input order preserved), list (unchecked asc / include_checked with checked last), update, delete, delete-checked-returning-count against `store.Querier`
- [x] 2.3 `service.go` — batch validation (1–200, name non-empty ≤300, offending index in error), products existence check via injected repo, checked_at stamping/clearing, sentinel errors
- [x] 2.4 `handlers.go` — POST bulk / GET / PATCH / DELETE single / DELETE ?checked=true (reject unqualified), swag annotations, `Register(rg)`

## 3. Wiring & tests

- [x] 3.1 Wire in `internal/httpserver` behind auth + idempotency; inject products repo
- [x] 3.2 Handler integration tests: bulk atomicity (bad index → zero rows), order preservation, default vs include_checked listing, check/uncheck stamps, clear-checked count, unqualified bulk delete 400, provenance SET NULL on product delete, unknown product 404

## 4. MCP & docs

- [x] 4.1 Register the five shopping tools with merge-before-call guidance in `add_shopping_items` description; bump expected-tools list
- [x] 4.2 `task swag`; `task vet` + `task test` green
