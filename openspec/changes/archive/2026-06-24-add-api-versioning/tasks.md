## 1. Server — relocate the domain group

- [x] 1.1 `internal/httpserver/server.go`: change `api := r.Group("/")` to `r.Group("/api/v1")`. Confirm `/healthz`, `/readyz`, `/swagger` stay registered on the engine (root, unversioned) and the `NoRoute` JSON-404 is unchanged.
- [x] 1.2 Verify every `Register(rg)` call hangs off the relocated group (no handler registered directly on the engine except infra).

## 2. MCP client

- [x] 2.1 `internal/mcpserver/apiclient.go`: ensure the `/api/v1` prefix is carried in `baseURL` and that `do()` appends the relative path rather than replacing the base path.
- [x] 2.2 Config default: `NUTRITION_API_URL` documentation/default becomes `http://localhost:8080/api/v1`.

## 3. Mobile companion

- [x] 3.1 Pairing: validate/store a `base_url` that includes `/api/v1`; confirm Dio joins relative paths against it without dropping the prefix.
- [x] 3.2 `Taskfile.yml` `dev:pair` + `prod:pair`: emit `base_url` with `/api/v1` in the QR payload.
- [x] 3.3 Note re-pair requirement in companion docs / pairing screen copy if applicable.

## 4. Garmin bridge

- [x] 4.1 `apps/garmin-bridge` config + httpx `base_url`: target `…/api/v1`; confirm relative paths (`/garmin/token`, `/garmin/sync-runs`, `/sync`) resolve correctly under the prefix.
- [x] 4.2 Update `NUTRITION_API_URL` in `.env.local` examples and `deploy/helm` values.

## 5. Docs + deploy

- [x] 5.1 Set `@BasePath /api/v1` on the swag root annotation; run `task swag`; commit regenerated `docs/`.
- [x] 5.2 Review `deploy/helm` for base-URL values and probe paths — probes stay at `/healthz`/`/readyz` (unchanged).
- [x] 5.3 Update `RUN_LOCAL.md` / `README.md` curl examples to `/api/v1`.

## 6. Tests + verification

- [x] 6.1 Update full-server integration tests (e.g. `internal/mcpserver/mcp_integration_test.go`) to the `/api/v1` prefix.
- [x] 6.2 Add an assertion that a representative endpoint is reachable under `/api/v1` and returns 404 at the old root path.
- [x] 6.3 Smoke each client's base-URL join with one live call (MCP, mobile, bridge) to catch path-reset bugs.
- [x] 6.4 `task test` green; `task vet` clean; `task swag` produces a clean diff. (Two packages — raceprep, recoverymetrics — hit the documented testcontainers parallel-boot `ping database: context deadline exceeded` flake under full-suite concurrency; both pass on isolated `-p 1` re-run. No assertion failures.)
