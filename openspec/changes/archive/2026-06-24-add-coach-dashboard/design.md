## Context

Kazper is one Cobra binary serving a Gin REST API (thin wrapper over Postgres) with two
clients: the Flutter companion and the MCP agent. After `add-api-versioning`, domain endpoints
live under `/api/v1` and root `/` is free. The read surface for a training dashboard already
exists as composites:

- `GET /api/v1/context/training` — phase, macrocycle/season + `days_to_race`, fitness snapshot
  (VO2max, `acute_load`, `chronic_load`, `training_status`, race predictions), derived `ACWR`,
  athlete config (FTP/zones), `watts_per_kg`, `recent_load`, `recent_workouts`,
  `upcoming_workouts`, coach memory.
- `GET /api/v1/context/recovery` — latest + recent recovery snapshots.

So the dashboard is a **presentation** project: ~2–3 GET calls, no new backend data.

The auth middleware (`internal/auth/middleware.go`) is Bearer-only today: a constant-time
compare of the token against `mobile`/`agent`/`garmin`, setting a `client_id` on context.

## Goals / Non-Goals

**Goals:**
- A wide, chart-dense, single-user training dashboard on a monitor, served from the binary.
- Same-origin (no prod CORS), authenticated by Basic auth the browser caches natively.
- Reuse the existing API; add no new data endpoints for v1.

**Non-Goals:**
- **Nutrition / fueling / EA panels.** Deferred — v1 is training-only.
- **Read-only `web` scoping.** v1 grants the `web` identity full access; a GET-only variant is
  a possible later change.
- **Multi-user / accounts / OAuth.** Single user; one Basic credential.
- **iOS/native.** This is a browser SPA.
- **Enforcing TLS in code.** Transport encryption (TLS/Tailscale) is an operational
  responsibility, documented not enforced.

## Decisions

### D1 — Basic auth as a second scheme in the existing middleware

`auth.Middleware` inspects the `Authorization` scheme: `Bearer ` → the existing token path
(unchanged); `Basic ` → base64-decode and constant-time compare against `WEB_USER:WEB_PASSWORD`,
resolving `client_id = "web"`. The `web` identity is OPTIONAL (recognized only when both env
vars are set), mirroring how `garmin` is gated. One realm covers the SPA shell and the API
calls it makes, so the browser prompts once and reuses the cached credential.

**Why Basic, not a token in `localStorage` or a pairing flow:** the browser holds the
credential natively and auto-attaches it on every same-origin request — the least machinery for
a single user, no refresh logic, no storage-leak surface beyond what the browser manages.

**Trade-off — full access in a browser:** a leaked credential can mutate data. Accepted for a
single-user tool; mitigated by encrypted transport. A read-only `web` identity (GET-only at the
middleware) remains an easy future tightening.

### D2 — SPA embedded via `embed.FS`, served at `/`, committed `dist/`

The built SPA is embedded in the binary (`embed.FS` over `apps/web/dist`) and served at `/`.
`dist/` is **committed**, mirroring the existing `docs/` precedent, so `go build` needs no Node
toolchain; `task web:build` regenerates it. This keeps the "one self-contained binary" shape
and the Helm deploy unchanged beyond two env vars.

**Alternative considered:** a Caddy reverse proxy doing Basic auth + static serving + bearer
injection, leaving the Go API untouched. Rejected for v1 — more infra, and the dashboard would
only work behind the proxy; embedding keeps it in the binary and "through the API."

### D3 — Routing: API under `/api/v1`, SPA owns the rest, infra untouched

Because the API moved to `/api/v1` (prerequisite), the SPA can own `/` without collision. Order
of resolution: `/api/v1/*` → API group; `/healthz`,`/readyz`,`/swagger` → infra; static asset
paths (`/assets/*`) → embedded files; any other GET → `index.html` (client-side routing). The
existing `NoRoute` JSON-404 must still fire for unknown `/api/v1/*` paths — the SPA fallback
applies to non-API GETs only, not to API 404s.

### D4 — Same-origin in prod, Vite proxy in dev

Prod is same-origin, so no CORS middleware is needed. Dev runs the Vite dev server (`:5173`)
proxying `/api` → `:8080`; CORS exists only in dev and is handled by the proxy. No CORS code
ships in the binary.

### D5 — Stack: TanStack Query + visx + Tailwind

TanStack Query polls the 2–3 context endpoints with a sane refetch interval (the data updates
at most daily via Garmin sync). visx renders the training charts (acute/chronic load trend,
ACWR gauge, load-by-sport bars) with full control over a coach's-eye aesthetic. Tailwind drives
the dense multi-panel monitor layout.

### D6 — v1 panels (training only)

```
┌ header: phase · season · days-to-race ─────────────────────────┐
│ ACWR / form gauge   ·   acute/chronic load trend               │
│ recovery snapshot (HRV · sleep · RHR)                          │
│ recent workouts (TSS, sport)   ·   upcoming workouts           │
└────────────────────────────────────────────────────────────────┘
```

Fueling / EA / protein-distribution panels are a deferred follow-on.

## Risks / Trade-offs

- **Basic auth over plaintext.** Credentials ride every request in base64. Mitigation: deploy
  behind TLS or Tailscale; document prominently; never expose the dashboard on a bare HTTP LAN
  endpoint.
- **Committed `dist/` drift.** The build artifact can fall out of sync with source. Mitigation:
  the `docs/` precedent already trains this discipline; a CI check (build + diff) can enforce it.
- **SPA fallback shadowing.** A greedy `index.html` catch-all could swallow API 404s or asset
  misses. Mitigation: scope the fallback to non-`/api/v1`, non-asset GETs; test that an unknown
  `/api/v1/...` still returns the JSON 404.
- **Bundle size / first paint on a monitor.** Acceptable for a single-user LAN/Tailscale tool;
  not optimizing for cold mobile loads.

## Migration Plan

Land after `add-api-versioning`. Ship: auth Basic path + `web` identity + config; `embed.FS`
serving + SPA fallback; `apps/web/` SPA + committed `dist/`; `web:build`/`web:dev` tasks; env +
Helm + docs. Operator sets `WEB_USER`/`WEB_PASSWORD` and reaches the dashboard over
TLS/Tailscale.

## Open Questions

- Refetch cadence — fixed interval vs. manual refresh vs. revalidate-on-focus? (Leaning:
  revalidate-on-focus + a slow background interval, since Garmin data lands ~daily.)
- Should the `web` identity be read-only from the start despite the "full access" decision, to
  shrink the browser-leak blast radius? (Captured as a deferred tightening, not v1.)
