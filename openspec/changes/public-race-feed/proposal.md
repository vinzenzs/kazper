## Why

The athlete wants a public "road to race" page to broadcast to friends — a single hero number counting down to the goal race. Kazper today has **zero** unauthenticated surface (every route is bearer- or basic-auth gated), and the public page will be a **separate frontend backed by Strapi**, not the embedded coach dashboard. To keep the athlete's sensitive data (weight, HRV, sleep, meals) fully private while still feeding the public page live from Kazper, Strapi acts as a shield: it holds one credential, pulls a deliberately tiny non-PII slice from Kazper on a schedule, caches it, and re-serves it publicly. This change adds only the Kazper-side seam that slice comes from — a single secret-gated, curated read endpoint. The Strapi instance and the public frontend are separate projects, out of scope here.

## What Changes

- A new **secret-gated public read endpoint** `GET /public/race-feed`, registered at the root path **outside** `auth.Middleware` (a sibling of `/healthz`/`/readyz`), returning only a curated, non-PII projection: `{race: {name, race_date}, days_remaining}`.
- Access is gated by a **single shared secret** (`X-Feed-Key: <FEED_SECRET>`), checked in constant time by the handler. This is **NOT** a bearer identity — it unlocks nothing else, so no route/identity authorization matrix (no RBAC) is introduced. Only Strapi holds the secret; it never reaches a browser.
- The feed resolves **the goal race as the active macrocycle's A-race** (`macrocycles.race_id`), reusing the existing model — no new "featured race" flag. `days_remaining` is computed against "today" in the configured user timezone.
- A new `FEED_SECRET` config var. When unset the endpoint is disabled (`503 feed_disabled`), mirroring the opt-in gating of the Garmin integration.
- The projection is intentionally minimal for v1; a live training-derived `progress` number (e.g. plan-adherence "31 of 48 sessions") is a **deferred follow-up**, not built here.

## Capabilities

### New Capabilities
- `public-race-feed`: a curated, secret-gated, unauthenticated read model that projects the active macrocycle's A-race (name, date) and a computed countdown for consumption by the external Strapi shield — the only public surface Kazper exposes.

### Modified Capabilities
<!-- none — reads existing macrocycle/race data, changes no existing requirement -->

## Impact

- **Code**: new `internal/publicfeed/` package (types/repo/service/handlers) reading the existing macrocycle + races tables read-only; wiring in `internal/httpserver/server.go` at the root engine (outside `auth.Middleware`), gated on `FEED_SECRET`; `internal/config/config.go` gains `FeedSecret` / `FEED_SECRET`.
- **API surface**: one new unauthenticated root route `GET /public/race-feed`; `task swag` re-run for its annotations. No change to any `/api/v1` route or the auth model.
- **Security**: the ONLY unauthenticated data route in the system; safe by construction (returns only race name/date/countdown — start-list-public information, no PII). Secret compared constant-time; disabled when unset.
- **External (out of scope, separate repos)**: the Strapi instance (content model + the scheduled pull holding `FEED_SECRET`) and the public frontend (SSR + OG-card image generation). Noted for context only.
- **Non-goals**: no `progress`/training-derived number (v2), no bearer identity or RBAC, no write path, no Strapi/frontend code, no change to existing capabilities.
