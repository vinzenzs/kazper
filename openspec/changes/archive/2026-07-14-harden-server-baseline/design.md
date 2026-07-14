## Context

Six loosely related baseline gaps from the 2026-07-13 analysis, consolidated deliberately (user request) rather than split into per-item changes. Current state:

- `internal/store/migrate.go` runs embedded migrations; a mid-migration failure leaves golang-migrate's `dirty` flag set, and with `MIGRATE_ON_START=true` (the default) every subsequent boot fails with the same raw error — no in-binary repair, the operator must hand-edit `schema_migrations`.
- `internal/httpserver/server.go` builds `http.Server` with only `ReadHeaderTimeout`; the `/api/v1` group stack is logger → auth → idempotency with no request deadline and no body cap; `logging.go` logs every completion at Info with no correlation id. Chat tool calls loop back through an in-process dispatcher, so one user turn produces N uncorrelatable log lines.
- No metrics of any kind. The deployment is a single pod behind Kubernetes — a Prometheus scrape target is the native fit.
- MCP parity: `PUT /athlete-config` is the only REST write without a tool (`athlete_config_get` exists). Precedent for PUT-backed tools is `set_goals`: full-replace semantics spelled out in the description, **no** idempotency key attached (the REST layer rejects `Idempotency-Key` on PUT per `harden-write-paths`), retry-unsafety acknowledged in the tool description.
- Known-red on `main`: MCP E2E `create_workout_template → not_found` (documented pre-existing in continuity). `internal/numfmt` is untested. `task test` never runs the race detector.

## Goals / Non-Goals

**Goals:**
- A dirty migration state is diagnosable and repairable from the shipped binary alone.
- Every request has a deadline, a bounded body, and a correlatable id; 5xx is loud in logs.
- Basic RED metrics (rate/errors/duration) scrapeable when enabled.
- The coach can update athlete physiology over MCP; the announced schema stays golden-gated.
- `main` is fully green, `numfmt` is tested, races are checkable on demand.

**Non-Goals:**
- Rate limiting and in-app TLS (single-user; TLS is the ingress/Tailscale layer's job — per the existing `coach-dashboard` decision).
- List-row caps (all list endpoints are date-window-gated; capping is a behavior change deferred until a real wide-window problem shows).
- Tracing/OpenTelemetry, dashboards, alerting — `/metrics` is the seam; consumers are out of scope.
- Backup freshness gauges (named in `add-data-backup` as a follow-up once this lands — wire it there, not here).

## Decisions

1. **`kazper migrate --force <version>` + dirty-state detection, not auto-repair.**
   Wrap golang-migrate's `Force(version)`. On any migrate failure (subcommand or `MIGRATE_ON_START`), query the `Version()` state and, when dirty, log the exact recovery command (`kazper migrate --force <version-1>` guidance plus a pointer to inspect the failed migration). Alternative — auto-forcing on boot — rejected: a dirty flag means a migration half-applied; blind auto-repair can mask real schema damage. Human-in-the-loop with a copy-pasteable command is the right altitude for a single-operator system.

2. **`athlete_config_update` follows `set_goals` verbatim.**
   Same registry pattern: full PUT payload struct mirroring the REST body, description spelling out full-replace semantics ("a field omitted is CLEARED") and PUT retry-unsafety, no idempotency key attached. **Tier: `write-confirm`** — unlike `set_goals` (write-auto), wrong FTP/zone values silently corrupt every subsequent workout-target resolution pushed to the watch, which is exactly the "training/goal writes pause for human confirm" tier definition. Registered in the same domain file as `athlete_config_get`; golden `announced_schemas.json` regenerated via `-tags=goldengen` (additive).

3. **Per-request timeout as `context.WithTimeout` middleware with a path-prefix exemption list, not `http.TimeoutHandler`.**
   `http.TimeoutHandler` buffers responses (breaks SSE) and returns an unstructured 503. A context-deadline middleware composes with the existing error shape (handlers already propagate `c.Request.Context()` into pgx) and lets exempt routes keep their own budgets. Exempt: `/api/v1/chat*` (SSE, owns `CHAT_REQUEST_TIMEOUT`), `/api/v1/meals/from_photo` (vision upstream), the Garmin proxy group (owns per-op budgets post-`garmin-bridge-call-resilience`). Default 30s via `HTTP_REQUEST_TIMEOUT`; a deadline-exceeded write maps to `504 request_timeout` when nothing has been written yet.

4. **Body cap via `http.MaxBytesReader` middleware, 1 MiB default, exemptions for self-capped routes.**
   Applied on `/api/v1` before binding; a `*http.MaxBytesError` from bind maps to `413 body_too_large` in the standard `{error: …}` shape. Exempt: meal photo (`MEAL_FROM_PHOTO_MAX_BYTES` owns it) and the Garmin library import (has its own cap in `internal/garmincontrol/library.go`). 1 MiB is ~50× the largest legitimate JSON body in the system (bulk workout ingest).

5. **Request id: honor-or-generate, echo, log, and forward through the loopback.**
   Middleware reads `X-Request-ID` (or generates a UUID), stores it in the Gin context, sets the response header, and `logging.go` includes it in every completion line — at **Error** level for status ≥500, Info otherwise. The chat loopback dispatcher sets the parent's request id on its synthetic subrequests, so one turn's tool fan-out shares one id. Alternative — full tracing — out of scope; the id gets 90% of the correlation value for 5% of the machinery.

6. **Metrics: promhttp at root, gated by `METRICS_ENABLED` (default false), histogram by route template.**
   Root-level like `/healthz` (infra, outside `auth.Middleware` — Prometheus scrapes shouldn't carry bearer tokens), but opt-in because root is ingress-exposed in production; the operator enables it only when the scrape path is private. Label by Gin's route *template* (`/api/v1/meals/:id`), never the raw path (cardinality). Standard Go runtime collectors come free with the default registry. Alternative — always-on with auth — rejected: Basic/bearer on scrape configs is more coupling than an env flag.

7. **The red E2E test is fixed inside this change, whatever it turns out to be.**
   It reproduces on a clean tree, so it's either a test-data drift or a real regression in template creation; both belong in a hardening sweep. If it uncovers a behavioral bug with spec implications, the fix lands here and the spec delta is added at archive time (precedent: decisions resolved mid-apply are recorded in the change).

## Risks / Trade-offs

- [30s deadline breaks an unanticipated slow route] → the exemption list is config-adjacent code reviewed in apply; `HTTP_REQUEST_TIMEOUT` can be raised per deployment; deadline-exceeded is loud in logs (Error + request id) so a mis-capped route is found in one occurrence.
- [1 MiB cap rejects a legitimate future payload] → `MAX_REQUEST_BODY_BYTES` is configurable; `413 body_too_large` is unambiguous to the caller.
- [`--force` misuse marks a half-applied migration clean] → the logged guidance says to inspect the failed migration and the down-file first; force requires an explicit version argument (no bare `--force`).
- [Metrics endpoint scraped through public ingress] → default-off; README documents "enable only with a private scrape path". No sensitive labels (route template, method, status only).
- [write-confirm tier makes `athlete_config_update` unusable from plain MCP (no confirm flow there)] → MCP-side tiers gate only the *chat* surface's pause; the MCP server dispatches all tiers directly (existing behavior for other write-confirm tools) — verified against the registry dispatch before landing.
- [Sweep-change breadth makes review harder] → tasks are grouped per workstream and independently verifiable; nothing shares state except config plumbing.

## Migration Plan

Single deploy. New env vars all have safe defaults (`HTTP_REQUEST_TIMEOUT=30s`, `MAX_REQUEST_BODY_BYTES=1MiB`, `METRICS_ENABLED=false`); behavior change for clients is only: very slow requests now 504, oversized bodies now 413, 5xx log level. Rollback = revert. `--force` is purely additive to the CLI.

## Open Questions

- None blocking. (The Helm chart could later grow a `metrics.serviceMonitor` toggle — deliberately left to a deploy-side follow-up, mirroring the `add-helm-fcm-push-config` precedent of keeping chart wiring in its own change.)
