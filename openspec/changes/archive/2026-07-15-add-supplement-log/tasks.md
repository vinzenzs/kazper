# Tasks

## 1. Schema & package

- [x] 1.1 Verify the on-disk migration head, then `task migrate:new NAME=add_supplement_entries` (**coordinate the slot** — sibling proposals also carry migrations); classify export-included in `dataexport/inventory.go`
- [x] 1.2 `internal/supplements/` per the template: types (paired dose omitempty), repo (create/get/window-asc/delete), service (sentinels: `name_required`, `dose_pair_required`, `dose_invalid`, `not_found`, range vocabulary + 92-day cap), handlers + wiring
- [x] 1.3 Integration tests: bare-name create, dose-pair matrix, window ascending/empty/range 400s, get/delete 404s, idempotent POST replay, no-PATCH-route
- [x] 1.4 `task swag`

## 2. Daily context

- [x] 2.1 Fold today's entries into `/context/daily` (`supplements` array, omitted when empty) + tests (present, omitted, payload otherwise unchanged)

## 3. MCP

- [x] 3.1 `log_supplement` (write) + `list_supplements` (read); golden regen (additive) + registry/integration green

## 4. Docs & verification

- [x] 4.1 README MCP table rows; `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 4.2 Live smoke: log via chat (write-confirm), read back in `/context/daily`, delete
