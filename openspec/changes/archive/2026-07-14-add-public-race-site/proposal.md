## Why

`public-race-feed` (archived 2026-07-14) shipped the backend seam: a secret-gated, non-PII `GET /public/race-feed` projection of the goal race — but the thing friends actually look at was deferred. The original topology assumed a Strapi shield (`friends → frontend → Strapi → X-Feed-Key → Kazper`); exploration concluded a **static site rebuilt on a schedule** is strictly less machinery and a *stronger* shield: at serve time there is no server, no secret, and no request path to Kazper at all. Strapi drops out of the picture (it only earns its keep for editorial content, which is not the v1 goal).

## What Changes

- New `apps/public/` — a small static "road to race" site (Astro): hero countdown, race name + date, a graceful off-season state, and a build-time-generated Open Graph share card.
- The build fetches `GET /public/race-feed` (with `X-Feed-Key` from CI secrets) at **build time only**; shipped output is pure static files containing no secret and making no runtime calls to Kazper.
- Countdown is computed client-side from the embedded `race_date` (prerendered `days_remaining` as the no-JS fallback), so a stale build can only be wrong about *which race*, never about *how many days*.
- New GitHub Actions workflow: nightly cron + manual dispatch + on-push rebuild, deploying to GitHub Pages. A failed build leaves the previous deploy serving.
- Supersedes decision D1 (Strapi-as-shield) of the archived `public-race-feed` design; the feed endpoint itself is unchanged.
- Docs: README pointer to the public site topology.

## Capabilities

### New Capabilities
- `public-race-site`: the static broadcast page's contract — build-time feed consumption, secret hygiene, client-side countdown freshness, null-state degradation, share card, and the scheduled rebuild pipeline.

### Modified Capabilities

_None._ `public-race-feed` requirements are untouched (the endpoint, its gating, and its projection are exactly what shipped); `deployment-pipeline` covers the backend image/chart workflows and gains no new requirement from an independent static-site workflow.

## Impact

- **New code:** `apps/public/` (Astro project — pages, countdown script, OG image generation, build-time fetch). No Go code, no migration, no REST/MCP surface change, no swag.
- **CI:** one new workflow file (`.github/workflows/public-site.yml`), repo secrets `FEED_SECRET` (already exists server-side; added to Actions secrets) and a `FEED_URL` variable; GitHub Pages enabled for the repo.
- **Ops:** production `FEED_SECRET` must be set (feed returns `503 feed_disabled` otherwise). Public traffic lands on Pages/CDN — never on the personal cluster.
- **Out of scope:** the v2 training-derived `progress` number, editorial content/CMS, custom domain (can be layered onto Pages later).
