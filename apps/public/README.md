# apps/public — "road to race" broadcast page

A tiny static site friends can visit to see the countdown to the goal race. It
is the public-facing half of the `public-race-feed` capability.

## Topology: CI is the shield (no runtime coupling to Kazper)

```
GitHub Actions (nightly cron) --X-Feed-Key--> Kazper GET /public/race-feed
        │  (build time only, once per build)
        ▼
   Astro static build  ──►  GitHub Pages  ──►  friends
```

At **serve time there is no server, no secret, and no path from public traffic
to Kazper** — the deployed output is pure static files. Kazper is touched only
once per rebuild, by CI. This supersedes the archived `public-race-feed`
design's Strapi-shield sketch: a static site rebuilt on a schedule is less
machinery and a stronger shield (Strapi only earned its keep for editorial
content, which is a non-goal).

The only volatile value — days remaining — is recomputed **client-side** from
the embedded `race_date` in the viewer's local timezone, so a stale build can
only be wrong about *which* race is featured, never *how many days*. The nightly
rebuild is a safety net, not a load-bearing dependency; a failed build leaves
the previous deploy serving.

## Build inputs (CI only)

| Variable | Where | Purpose |
|---|---|---|
| `FEED_URL` | Actions **variable** | Full URL of `GET /public/race-feed` (e.g. `https://kazper.example.com/public/race-feed`) |
| `FEED_SECRET` | Actions **secret** | Value of the server's `FEED_SECRET`, sent as `X-Feed-Key` at build time only |
| `SITE_URL` / `BASE_PATH` | Actions (set by the Pages step) | Absolute URLs for the `og:image` tag |

The secret is used only by `scripts/fetch-feed.mjs` at build time and never
enters the shipped assets (`feed.json` carries only `{race, days_remaining}`).

## Local build

```bash
cd apps/public
npm install
FEED_URL=… FEED_SECRET=… npm run build   # → dist/ (fetch → astro build → OG card)
npm run preview                          # serve dist/ locally
```

`npm run build` runs `fetch-feed` (writes `src/generated/feed.json`, failing the
build on any non-200), then `astro build`, then `gen-og` (writes `dist/og.png`).
A `{race: null}` feed builds the designed off-season page. `dist/` is **not**
committed (unlike `apps/web`, whose dist is embedded in the Go binary) — CI
deploys it to Pages.

Deployment is `.github/workflows/public-site.yml` (nightly cron +
`workflow_dispatch` + pushes under `apps/public/**`).
