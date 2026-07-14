## Why

The 2026-07-13 gap analysis surfaced a cluster of small-to-medium baseline gaps that don't individually justify a change each but together leave the server less operable than the feature surface deserves: a failed startup migration boot-loops with no in-binary recovery, a hung request has no deadline, request bodies are unbounded, 5xx responses log at Info with no request id to correlate by, there is zero metrics visibility, the coach cannot write athlete physiology config over MCP (the one verified REST↔MCP parity gap), one MCP E2E test is known-red on `main`, and `internal/numfmt` — the rounding layer under every nutrient response — has no tests. This change sweeps them in one pass.

## What Changes

- **Migration dirty-state recovery**: `kazper migrate` gains a `--force <version>` flag (wrapping golang-migrate's `Force`) to clear a dirty flag after a failed migration, and both `migrate` and the `MIGRATE_ON_START` path SHALL detect the dirty state and log an actionable message naming the recovery command — instead of today's bare failure boot-loop.
- **MCP parity — `athlete_config_update`**: a write tool wrapping `PUT /athlete-config`, following the `set_goals` PUT precedent (full-replace semantics, no idempotency key — PUT rejects it); `athlete_config_get` already exists. Golden announced-schema regenerated.
- **HTTP server hardening**:
  - `http.Server` gains `ReadTimeout` and `IdleTimeout` (only `ReadHeaderTimeout` is set today; `WriteTimeout` stays unset for SSE).
  - A per-request timeout middleware on `/api/v1` (config `HTTP_REQUEST_TIMEOUT`, default 30s) with an explicit exemption list for streaming/long routes (`/chat`, `/chat/confirm`, `/meals/from_photo`, the Garmin proxy group which owns its budgets post-`garmin-bridge-call-resilience`).
  - A global request-body size cap (config `MAX_REQUEST_BODY_BYTES`, default 1 MiB) returning `413 body_too_large`, exempting routes that already carry their own caps (meal photo, Garmin library import).
  - Request-id middleware: honor inbound `X-Request-ID` or generate one, echo it on the response, include it in the request log, and forward it through the chat loopback dispatcher so tool-call subrequests correlate to their parent turn. 5xx responses log at **Error** level (Info today).
- **Opt-in Prometheus metrics**: `GET /metrics` (promhttp) registered at the root infra group, enabled only when `METRICS_ENABLED=true` (default false); a Gin middleware records request count + duration histograms by route/method/status. New `prometheus/client_golang` dependency.
- **Test hygiene (no spec deltas)**: unit tests for `internal/numfmt`; fix the pre-existing red MCP E2E `create_workout_template → not_found`; add a `task test:race` target.
- **Cleanups (no spec deltas)**: remove the accepted-but-unused root `--config` flag; guard `task install`'s `codesign` step behind a darwin check.

## Capabilities

### New Capabilities
- `server-hardening`: the HTTP runtime baseline — server/request timeouts, body-size cap, request-id correlation with error-level 5xx logging, and the opt-in Prometheus metrics endpoint.

### Modified Capabilities
- `cli`: ADDED requirement — the migrate subcommand detects a dirty migration state, reports it actionably, and recovers via `--force <version>`.
- `athlete-config`: ADDED requirement — an `athlete_config_update` MCP tool mirrors `PUT /athlete-config` (the existing "rejects an idempotency key" requirement is unchanged and the tool honors it by sending none).

## Impact

- **Code**: `cmd/kazper/migrate.go` + `internal/store/migrate.go` (force/dirty detection); `internal/agenttools/registry_garmin_inventory.go` or a new registry file (update tool) + regenerated `testdata/announced_schemas.json`; `internal/httpserver/server.go` (middleware wiring, `http.Server` fields, `/metrics`); `internal/httpserver/logging.go` (request id, 5xx level); `internal/chat` dispatcher (forward request id); `internal/config/config.go` (`HTTP_REQUEST_TIMEOUT`, `MAX_REQUEST_BODY_BYTES`, `METRICS_ENABLED`); `internal/numfmt` tests; `Taskfile.yml` (`test:race`, codesign guard); `cmd/kazper/main.go` (drop `--config`).
- **Dependencies**: adds `github.com/prometheus/client_golang`.
- **API surface**: one new opt-in infra route (`/metrics`, root-level like `/healthz`); one new error code (`body_too_large`); no `/api/v1` route changes. `task swag` re-run (annotations for the error shape only if surfaced in handler structs).
- **Docs**: README config table gains the three new env vars.
- **Explicit non-goals**: rate limiting, in-app TLS termination, list-row caps, tracing/OpenTelemetry.
