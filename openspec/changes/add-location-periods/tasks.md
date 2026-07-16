# Tasks

## 1. Schema & package

- [ ] 1.1 Verify migration head (sibling proposals also carry migrations — coordinate slots), then `task migrate:new NAME=add_location_periods`; export-included classification in `dataexport/inventory.go`
- [ ] 1.2 `internal/locations/` per the template: types/repo/service/handlers (validation matrix, no PATCH, overlap-tolerant window read); `HOME_LAT`/`HOME_LON` config pair (both-or-neither validation)
- [ ] 1.3 `LocationOn(date)` resolution primitive + `GET /locations/resolve` (travel/home/unconfigured, latest-start tie-break)
- [ ] 1.4 Integration tests: CRUD matrix, overlap tie-break, home fallback, unconfigured 404, resolve-vs-primitive consistency, idempotent POST replay
- [ ] 1.5 `task swag`

## 2. MCP

- [ ] 2.1 `log_location_period` (write) + `list_location_periods` (read); golden regen (additive) + registry/integration green

## 3. Verification

- [ ] 3.1 `task vet` + full suite green (`-p 1` on boot-contention flakes); live: log a real upcoming trip via chat, resolve dates inside/outside it
