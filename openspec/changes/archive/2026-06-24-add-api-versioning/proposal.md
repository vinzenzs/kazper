## Why

Every domain endpoint is served at the root path — `api := r.Group("/")` in
`internal/httpserver/server.go` — with no version prefix. That was fine for three first-party
clients, but it has two costs now: (a) there is no room to evolve the HTTP contract without
breaking callers, and (b) the root path `/` is already "owned" by the API, which blocks a
browser SPA from being served same-origin. The follow-on change `add-coach-dashboard` needs
`/` for a web dashboard embedded in the same binary.

This change relocates all versioned domain endpoints under `/api/v1`, freeing root for the SPA
and introducing real API versioning. It is a **behavior-preserving cutover**: same handlers,
same auth, same payloads, same error shapes — only the path prefix changes. Infra endpoints
(`/healthz`, `/readyz`) and the docs UI (`/swagger`) stay unversioned, because liveness probes
and documentation are not part of the versioned contract.

## What Changes

- **Server**: change the domain route group from `r.Group("/")` to `r.Group("/api/v1")`.
  Health/readiness probes and Swagger UI remain at root. The `NoRoute` JSON-404 behavior is
  unchanged. Because every per-package handler test mounts its own `r.Group("/")` and hits
  paths directly, those tests are unaffected; only full-server integration tests traverse the
  real prefix.
- **MCP server**: the API client base URL gains the `/api/v1` segment at its single join point
  (`internal/mcpserver/apiclient.go`); every tool still issues exactly one HTTP call with the
  same relative path.
- **Mobile companion**: the stored base URL and the pairing QR payload (`dev:pair` /
  `prod:pair` Taskfile tasks) target `…/api/v1`. Already-paired devices must re-pair.
- **Garmin bridge**: the backend base URL (`NUTRITION_API_URL`) targets `…/api/v1`; the
  bridge's relative paths (`/garmin/token`, `/garmin/sync-runs`) are unchanged.
- **Docs**: set `@BasePath /api/v1` and regenerate `docs/`.
- **Breaking, all-at-once**: root domain routes are removed in the same change — there is no
  deprecation window. Acceptable for a single-user monorepo where all clients ship together.

## Capabilities

### New Capabilities
<!-- none — this is a relocation of the existing HTTP surface, not new behavior -->

### Modified Capabilities

- `api-docs`: the OpenAPI base path SHALL be `/api/v1`; documented domain endpoints are served
  there while infra endpoints (`/healthz`, `/readyz`, `/swagger`) remain at root.
- `mcp-server`: the API client SHALL target the `/api/v1` base; `NUTRITION_API_URL`'s default
  becomes `http://localhost:8080/api/v1`.
- `mobile-companion`: the pairing payload's `base_url` SHALL include the `/api/v1` prefix.
- `garmin-bridge`: the bridge's REST calls SHALL target the `/api/v1` base.

## Impact

- **Backend** (`internal/httpserver/server.go`): one route-group change; full-server
  integration tests (e.g. `internal/mcpserver/mcp_integration_test.go`) updated to the new
  prefix. Per-package handler tests untouched.
- **MCP** (`internal/mcpserver/apiclient.go`, config default): base URL carries `/api/v1`.
- **Mobile** (`apps/companion`): the pairing flow validates/stores a `/api/v1` base; the
  `dev:pair`/`prod:pair` Taskfile payloads emit it. **Re-pair required.**
- **Bridge** (`apps/garmin-bridge`): `NUTRITION_API_URL` / Helm values + `.env.local` examples
  point at `/api/v1`.
- **Docs**: `docs/` regenerated with the new base path (`task swag`).
- **Deploy** (`deploy/helm`): any base-URL values and probe paths reviewed — probes stay at
  `/healthz`/`/readyz` (unversioned), so liveness/readiness config is unchanged.
- **No data, auth, or payload changes** — purely a path relocation. This change is a
  prerequisite for `add-coach-dashboard`.
