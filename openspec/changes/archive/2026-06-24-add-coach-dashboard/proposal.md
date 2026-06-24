## Why

Kazper has two clients — a capture-first mobile app and a conversational MCP agent — but no
surface for *sitting at a monitor and reading training trends at a glance*. The data already
exists: `GET /context/training` is effectively a coach's-eye composite (phase, season,
days-to-race, fitness snapshot, acute/chronic load, ACWR, athlete zones, watts/kg, recent +
upcoming workouts), and `GET /context/recovery` adds the recovery picture. What's missing is
**presentation** — a wide, chart-dense dashboard that no phone screen or chat transcript can
match.

This change adds a single-user web dashboard: a fresh Vite + React + TypeScript SPA, served
**same-origin** from the Kazper binary (embedded via `embed.FS`), reading the existing REST
API. v1 is **training-only** — the fueling/EA panels are deferred to a later change.

Depends on `add-api-versioning` (domain endpoints under `/api/v1`, which frees `/` for the
SPA).

## What Changes

- **Auth — Basic auth for the browser.** `auth.Middleware` learns `Authorization: Basic` in
  addition to `Bearer`: a credential matching the configured `WEB_USER` / `WEB_PASSWORD`
  resolves to a new `web` client identity with **full access** (same authorization as mobile;
  no read-only restriction in v1). Bearer behavior is unchanged. Basic is base64, not
  encryption, so it is only safe over an encrypted transport — the dashboard is intended to be
  reached over TLS or Tailscale; this is documented, not enforced in code.
- **Serving — SPA embedded at root.** The built SPA (`apps/web/dist`) is embedded with
  `embed.FS` and served at `/`; unmatched non-API GET paths return `index.html` (client-side
  routing). API lives under `/api/v1` (from the prerequisite), so there is no route collision
  and the same-origin model means **no CORS in production**.
- **The SPA — `apps/web/`.** Vite + React + TypeScript, TanStack Query polling the context
  endpoints, **visx** for charts, **Tailwind** for the dense monitor layout. v1 panels
  (training only): header (phase · season · days-to-race), ACWR/form gauge, acute/chronic load
  trend, recovery snapshot (HRV · sleep · RHR), and recent + upcoming workouts.
- **Build integration.** A `task web:build` runs `vite build` into `apps/web/dist`, which is
  **committed** (mirroring the `docs/` precedent) so plain `go build` needs no Node toolchain.
  Dev uses the Vite dev server proxying to `:8080` (CORS only in dev, harmless).
- **No new data endpoints.** v1 reads `/api/v1/context/training`, `/api/v1/context/recovery`,
  and existing workout reads. Any aggregation is presentation-side.

## Capabilities

### New Capabilities

- `coach-dashboard`: a single-user, same-origin web dashboard that reads the REST API and
  presents a training-focused coach's-eye view, authenticated by HTTP Basic auth.

### Modified Capabilities

- `auth`: a new `web` client identity authenticated via HTTP Basic auth (`WEB_USER` /
  `WEB_PASSWORD`), with full access; Bearer identities are unchanged.

## Impact

- **Backend** (`internal/auth`): Basic-auth path + `web` identity + `WEB_USER`/`WEB_PASSWORD`
  config (startup validation consistent with other optional identities — present only when
  both are set). `internal/httpserver/server.go`: `embed.FS` static serving at `/` with an
  `index.html` SPA fallback that does not shadow `/api/v1` or the infra/`NoRoute` 404 contract.
- **New app** (`apps/web/`): the Vite + React + TS + visx + Tailwind SPA and its `dist/`
  (committed). New `Taskfile.yml` targets: `web:build`, `web:dev`.
- **Config/docs**: `.env.local` example + `README`/`RUN_LOCAL` gain `WEB_USER`/`WEB_PASSWORD`
  and a "reach it over TLS/Tailscale" note; `deploy/helm` exposes the two env vars.
- **Scope**: training-only v1. Fueling / energy-availability / protein-distribution panels and
  any read-only `web` scoping are explicitly deferred to a follow-on change.
- **Security note**: a full-access credential in a browser can mutate data if leaked; this is
  an accepted single-user trade-off, mitigated by encrypted transport (TLS/Tailscale).
