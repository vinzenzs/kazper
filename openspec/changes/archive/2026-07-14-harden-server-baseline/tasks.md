## 1. Migration dirty-state recovery

- [x] 1.1 `internal/store/migrate.go`: expose dirty-state detection (wrap `Version()`) and a `Force(version)` path; on failure, wrap the error with dirty version + recovery guidance when dirty
- [x] 1.2 `cmd/kazper/migrate.go`: add `--force <version>` (reject bare `--force`); `serve`'s `MIGRATE_ON_START` failure path surfaces the same actionable message
- [x] 1.3 Tests in `internal/store`: simulate a dirty state (insert into `schema_migrations` with `dirty=true` on the testcontainer), assert detection message and that `Force` + re-migrate recovers

## 2. MCP parity — athlete_config_update

- [x] 2.1 Add `AthleteConfigUpdateArgs` (mirroring the PUT body) + `athlete_config_update` spec (Tier `write-confirm`, no idempotency key, full-replace description) next to `athlete_config_get`
- [x] 2.2 Verify MCP-side dispatch of write-confirm tools executes directly (chat-only pause) — matches existing write-confirm tools
- [x] 2.3 Regenerate `announced_schemas.json` via `-tags=goldengen`; golden + MCP integration tests green
- [x] 2.4 Registry unit test: build produces `PUT /athlete-config` with no `Idempotency-Key`

## 3. HTTP server hardening

- [x] 3.1 `internal/config`: add `HTTP_REQUEST_TIMEOUT` (30s), `MAX_REQUEST_BODY_BYTES` (1 MiB), `METRICS_ENABLED` (false); README config table
- [x] 3.2 `http.Server`: set `ReadTimeout` + `IdleTimeout` (leave `WriteTimeout` unset for SSE)
- [x] 3.3 Request-timeout middleware on `/api/v1` with path-prefix exemptions (`/chat`, `/chat/confirm`, `/meals/from_photo`, Garmin proxy group); deadline before first write → `504 request_timeout`
- [x] 3.4 Body-cap middleware (`http.MaxBytesReader`) with exemptions (meal photo, Garmin library import); `*http.MaxBytesError` → `413 body_too_large`
- [x] 3.5 Request-id middleware (honor/generate `X-Request-ID`, echo header, Gin context); `logging.go` includes id and logs status ≥500 at Error
- [x] 3.6 Chat loopback dispatcher forwards the parent request id onto tool subrequests
- [x] 3.7 httpserver tests: timeout on a stubbed slow handler, 413 on oversized body, photo-route exemption, id echo + propagation, 5xx log level

## 4. Opt-in metrics

- [x] 4.1 Add `prometheus/client_golang`; request count + duration histogram middleware labeled by route template/method/status class
- [x] 4.2 Register `GET /metrics` (promhttp) at the root infra group only when `METRICS_ENABLED=true`
- [x] 4.3 Tests: 404 when disabled; enabled exposition contains route-template-labeled series (no raw ids)

## 5. Test hygiene

- [x] 5.1 Unit tests for `internal/numfmt` (rounding semantics incl. halfway cases, negative values, NaN/Inf guards if applicable)
- [x] 5.2 Diagnose and fix the red MCP E2E `create_workout_template → not_found`; if the fix has spec implications, record the decision in this change for archive-time sync — **it was test-data drift, not a behavioral bug: the E2E set `NUTRITION_API_URL` to the bare origin, missing the `/api/v1` prefix the MCP client requires post-`add-api-versioning`, so every tool call 404'd. Fixed in the test (`+config.APIBasePath`). No spec delta.**
- [x] 5.3 Add `task test:race` (full suite with `-race`; document the longer runtime)

## 6. Cleanups

- [x] 6.1 Remove the unused root `--config` flag from `cmd/kazper/main.go`
- [x] 6.2 Guard `task install`'s `codesign` step behind an OS check (darwin only)

## 7. Verification & wrap-up

- [x] 7.1 `task test`, `task vet`, `task swag` (new error code annotations if surfaced in handler structs), MCP golden + integration, `task test:race` once — vet clean; swag no drift (middleware errors aren't handler-annotated); full suite 52 pkg green (the lone `goals` failure was testcontainers boot contention — passes `-p 1` alone); MCP golden + integration (E2E + announced-tools) green; `-race` clean on the touched packages
- [x] 7.2 Update task states and propose the `feat(server): harden runtime baseline` commit
