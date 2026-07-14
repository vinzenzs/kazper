# apps/public — "road to race" broadcast page

A tiny static site friends can visit to see the countdown to the goal race. It
is the public-facing half of the `public-race-feed` capability.

## Topology: CI-built image, served in-cluster (no runtime coupling to Kazper)

```
GitHub Actions (nightly cron) --X-Feed-Key--> Kazper GET /public/race-feed
        │  (IMAGE-build time only, once per build; secret is a BuildKit secret)
        ▼
   Astro static build  ──►  nginx image (GHCR)  ──►  k8s Deployment (Helm)  ──►  friends
```

The static build is baked into a tiny `nginx` container (`Dockerfile`), pushed
to GHCR, and served in-cluster via the chart's opt-in `publicSite` block (its own
public Ingress host with **no route to the API**). At **serve time the feed
secret is not in the cluster** (it is a BuildKit build secret, never an image
layer) and public traffic reaches only a static file server. Kazper is touched
once per rebuild, by CI. This supersedes the archived `public-race-feed`
design's Strapi-shield sketch and the earlier GitHub Pages topology
(`host-public-site-in-cluster`) — one deploy surface, the cluster the athlete
already runs.

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
| `SITE_URL` / `BASE_PATH` | build args (the public host; `BASE_PATH=/`) | Absolute URLs for the `og:image` tag |

The secret is used only by `scripts/fetch-feed.mjs` at build time and never
enters the shipped assets or any image layer (`feed.json` carries only
`{race, days_remaining}`; the `Dockerfile` passes `FEED_SECRET` via
`--mount=type=secret`).

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
committed (unlike `apps/web`, whose dist is embedded in the Go binary).

## Container image + deploy

The `Dockerfile` bakes the static build into an `nginx-unprivileged` image
(non-root, port 8080). Local build (from the repo root):

```bash
FEED_SECRET=… docker build -f apps/public/Dockerfile \
  --secret id=feed_secret,env=FEED_SECRET \
  --build-arg FEED_URL="https://kazper.example.com/public/race-feed" \
  --build-arg SITE_URL="https://race.example.com" \
  -t kazper-public-site apps/public
```

`.github/workflows/public-site.yml` (nightly cron + `workflow_dispatch` + pushes
under `apps/public/**`) builds + pushes the image to GHCR and rolls the
in-cluster Deployment. Enable serving via the Helm chart's `publicSite` block —
see `deploy/helm/kazper/README.md`.
