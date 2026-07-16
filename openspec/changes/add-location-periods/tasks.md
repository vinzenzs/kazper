# Tasks

## 1. Schema & package

- [x] 1.1 Verify migration head (sibling proposals also carry migrations — coordinate slots), then `task migrate:new NAME=add_location_periods`; export-included classification in `dataexport/inventory.go`
      _Head re-checked at apply, as the design's coordinate-slots note asks: `add-workout-environment` (the sibling shipped earlier this session) took **065**, so this took **066**. Scaffold's 6-digit prefix renamed to 3-digit to match the head._
- [x] 1.2 `internal/locations/` per the template: types/repo/service/handlers (validation matrix, no PATCH, overlap-tolerant window read); `HOME_LAT`/`HOME_LON` config pair (both-or-neither validation)
      _`HOME_LAT`/`HOME_LON` are held as **strings**, not floats: 0 is a real coordinate (Null Island), so a numeric zero value could not be told apart from "unset". `HomeLocation()` returns `(lat, lon, ok, err)` — `ok=false` for a legitimately unset pair, error for one-sided or malformed; `ValidateForServe` rejects the latter at boot (the `WEB_USER`/`WEB_PASSWORD` pattern), so a booted server only ever sees "unset" or "valid". Same reasoning drives `*float64` on the create input._
- [x] 1.3 `LocationOn(date)` resolution primitive + `GET /locations/resolve` (travel/home/unconfigured, latest-start tie-break)
      _`created_at DESC` added as the secondary tie-break for two periods sharing a `start_date` — the more recently logged is the more recent intent. The spec's latest-start rule alone leaves that case to chance._
- [x] 1.4 Integration tests: CRUD matrix, overlap tie-break, home fallback, unconfigured 404, resolve-vs-primitive consistency, idempotent POST replay
      _Idempotent replay is covered centrally by the idempotency middleware (this handler adds no key handling of its own — the `Idempotency-Key` header is the middleware's, per the repo's write convention), so the package test asserts the endpoint contract rather than re-testing the middleware. Added beyond the list: 0,0-coordinates-are-valid, inclusive-on-both-ends, insert-order-independence of the tie-break, partial-overlap window reads, PATCH-not-routed, and the delete + re-log extension path._
- [x] 1.5 `task swag`

## 2. MCP

- [x] 2.1 `log_location_period` (write) + `list_location_periods` (read); golden regen (additive) + registry/integration green

## 3. Verification

- [x] 3.1 `task vet` + full suite green (first run, no flake)
- [ ] 3.1b Live: log a real upcoming trip via chat, resolve dates inside/outside it
      _(operator step — needs the live MCP client; not runnable in-session. Note `HOME_LAT`/`HOME_LON` must be set in the deployment env, or every non-travel date resolves `location_unconfigured` and the heat reads degrade.)_
