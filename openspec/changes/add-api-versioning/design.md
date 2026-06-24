## Context

The Gin engine builds one domain route group at root (`api := r.Group("/")` in
`internal/httpserver/server.go`); `/healthz`, `/readyz`, and `/swagger` are registered
directly on the engine outside that group, and an `r.NoRoute` handler returns a JSON 404.
Three first-party clients call the API, each joining request paths against a single
configurable base:

- **MCP** — `apiClient{ baseURL *url.URL }` (`internal/mcpserver/apiclient.go`); default
  `http://localhost:8080`.
- **Mobile** — Dio `baseUrl` pulled from the paired QR payload `{base_url, token}`.
- **Garmin bridge** — httpx `Client(base_url=NUTRITION_API_URL)` with relative paths.

Critically, **per-package handler tests mount their own `r.Group("/")`** and request
`/summary/daily` etc. directly, so they are insensitive to the server's prefix; only
full-server integration tests traverse the real mount point.

This change relocates the domain group to `/api/v1` to free `/` for an embedded SPA
(`add-coach-dashboard`) and to introduce versioning. No behavior changes.

## Goals / Non-Goals

**Goals:**
- All domain endpoints served under `/api/v1`; root `/` freed for a future SPA.
- Infra endpoints (`/healthz`, `/readyz`, `/swagger`) stay unversioned.
- All three clients + docs updated in lockstep so nothing 404s after the cutover.

**Non-Goals:**
- **A deprecation window / dual-mount.** No simultaneous root + `/api/v1` serving.
- **`/api/v2` or any contract change.** Same handlers, payloads, auth, error shapes.
- **Per-endpoint versioning.** One flat `v1` prefix for the whole domain surface.
- **Touching per-package handler tests.** They mount their own root group; leave them.

## Decisions

### D1 — A single route-group prefix, nothing per-handler

Change `r.Group("/")` to `r.Group("/api/v1")`; every `Register(rg)` call already hangs off
that group, so handlers move as a set with no per-package edits. Relative handler paths
(`/meals`, `/garmin/sync-runs`) are unchanged.

**Alternative considered:** move the API under `/api` and version inside each capability.
Rejected — more surface, no benefit for a single-user API with one live version.

### D2 — Infra endpoints stay at root, unversioned

`/healthz`, `/readyz`, and `/swagger` remain on the engine, not under `/api/v1`. Kubernetes
liveness/readiness probes and the docs UI are operational concerns, not part of the versioned
contract; versioning them would churn Helm probe config for no gain.

### D3 — Each client carries the prefix in its base URL, not per-call

The prefix lives in one place per client (MCP `baseURL`, Dio `baseUrl`, httpx `base_url`), so
relative request paths stay identical. Watch the join semantics: httpx and Dio reset the path
when a request path begins with `/` against a base that has a path component — the base must
include `/api/v1` **and** the join must append rather than replace. A post-cutover smoke test
asserts a real request resolves to `…/api/v1/…`.

### D4 — All-at-once cutover, no dual-mount

Root domain routes are removed the moment `/api/v1` lands. In a single-user monorepo every
client is updated and shipped together, so a deprecation window is needless complexity. (For a
multi-tenant API the alternative would be to mount both prefixes for a release, then drop root
— explicitly out of scope here.)

### D5 — Pairing payload changes ⇒ re-pair

The QR payload's `base_url` now includes `/api/v1`, so already-paired devices must re-pair (or
have their stored base URL migrated). The `dev:pair`/`prod:pair` tasks emit the new value;
re-pairing is a one-scan operation and is called out in the proposal Impact.

## Risks / Trade-offs

- **A client path is missed → 404s.** Mitigation: an integration test asserts a representative
  endpoint is reachable under `/api/v1` and returns 404 at root; grep for any hardcoded
  root-absolute paths in each client before shipping.
- **base_url join subtleties (Dio/httpx).** A leading-slash request path can discard the
  base's `/api/v1`. Mitigation: verify the join in each client with one live call in the smoke
  test; prefer base URLs without a trailing slash plus relative paths joined correctly.
- **Stale deploy config.** Helm/env base URLs or a reverse proxy may still point at root.
  Mitigation: review `deploy/helm` values and `.env*` examples as part of this change.

## Migration Plan

Ship as one commit: server group change + MCP base + mobile pairing/base + bridge base +
`task swag` regen + integration-test updates + Helm/`.env` examples. Then: re-pair the mobile
app, update the bridge's `NUTRITION_API_URL`, redeploy. Probes need no change (unversioned).

## Open Questions

- Should `/swagger` move under `/api/v1/docs` for tidiness, or stay at root? (Leaning: stay at
  root with the other infra endpoints — D2.)
