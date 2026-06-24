## 1. Auth — Basic auth + `web` identity

- [x] 1.1 `internal/auth/middleware.go`: add a `Basic ` branch alongside `Bearer `; base64-decode and constant-time compare against `WEB_USER:WEB_PASSWORD`; on match set `client_id = "web"`. Bearer behavior unchanged.
- [x] 1.2 Gate the `web` identity on both env vars being set (recognized only when configured, mirroring `garmin`).
- [x] 1.3 Config: add `WEB_USER` / `WEB_PASSWORD` (Viper) + startup validation consistent with optional identities. Full access (no GET-only restriction in v1).
- [x] 1.4 Auth tests: Basic credential → `web` identity accepted; wrong/absent Basic → 401; Bearer paths unaffected; `web` unrecognized when env unset.

## 2. Serving — embed the SPA at `/`

- [x] 2.1 `embed.FS` over `apps/web/dist`; serve `/assets/*` and other static files from it.
- [x] 2.2 SPA fallback: unmatched non-`/api/v1`, non-asset GETs return `index.html`; confirm unknown `/api/v1/*` still returns the JSON `NoRoute` 404 and infra endpoints are untouched.
- [x] 2.3 Apply the Basic-auth realm to the SPA shell + its API calls (one realm, browser prompts once).

## 3. SPA scaffold — `apps/web/`

- [x] 3.1 Vite + React + TypeScript project; Tailwind configured; visx + TanStack Query added.
- [x] 3.2 API layer: typed client + TanStack Query hooks for `/api/v1/context/training` and `/api/v1/context/recovery`; refetch policy (revalidate-on-focus + slow interval).
- [x] 3.3 Vite dev proxy `/api` → `http://localhost:8080` for `web:dev`.

## 4. Dashboard panels (training only)

- [x] 4.1 Header: phase · season name · `days_to_race`.
- [x] 4.2 ACWR / form gauge from the derived `acwr`.
- [x] 4.3 Acute/chronic load trend chart (visx).
- [x] 4.4 Recovery snapshot (HRV · sleep · RHR) from `/context/recovery`.
- [x] 4.5 Recent + upcoming workouts lists (sport, status, TSS).
- [x] 4.6 Empty/loading/error states for each panel; graceful nulls (every metric is nullable).

## 5. Build integration

- [x] 5.1 `Taskfile.yml`: `web:build` (`vite build` → `apps/web/dist`) and `web:dev` (Vite dev server).
- [x] 5.2 Commit `apps/web/dist` (mirror the `docs/` precedent) so `go build` needs no Node; document the regenerate step.
- [x] 5.3 Ensure `task build` works without the Node toolchain present (embedded committed dist).

## 6. Config, deploy, docs

- [x] 6.1 `.env.local` example + `README`/`RUN_LOCAL`: `WEB_USER` / `WEB_PASSWORD` + a "reach it over TLS/Tailscale (Basic auth is not encryption)" note.
- [x] 6.2 `deploy/helm`: expose `WEB_USER` / `WEB_PASSWORD`; document the transport expectation.

## 7. Tests + verification

- [x] 7.1 Backend: serving test — `/` returns the SPA shell, `/assets/*` serves files, an unknown SPA route returns `index.html`, an unknown `/api/v1/*` returns the JSON 404.
- [x] 7.2 Backend: Basic auth covered in 1.4.
- [x] 7.3 SPA: a render test per panel against fixture context payloads (including null-heavy snapshots).
- [x] 7.4 `task test` + `task vet` green; `web:build` produces a committed-clean `dist/` diff.
- [x] 7.5 Manual smoke (documented): set `WEB_USER`/`WEB_PASSWORD`, reach `/` over Tailscale, confirm the browser Basic prompt and that all training panels render from live data.
  - Automated/live-binary smoke performed: ran the compiled `kazper serve` with `WEB_USER`/`WEB_PASSWORD` set and verified over real HTTP that `GET /` (no creds) → `401` + `WWW-Authenticate: Basic realm="Kazper Coach"` (browser prompts), `GET /` (with creds) → `200` embedded SPA shell, and `GET /api/v1/nope` (no creds) → JSON `404` (API contract preserved). Also covered by `TestSPA_RealEmbeddedBuildServes` against the actual `go:embed` build.
  - Remaining operator step (manual, environment-specific): reach `/` in a browser over Tailscale/TLS and confirm the native Basic prompt + that the training panels render from live Garmin-fed data. Documented in `RUN_LOCAL.md` (Coach dashboard) and `README.md`.
