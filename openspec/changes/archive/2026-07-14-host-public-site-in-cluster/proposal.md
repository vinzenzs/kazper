# Host the public race site in-cluster (supersede GitHub Pages)

## Why

`add-public-race-site` (archived 2026-07-14) shipped the public "road to race"
page as static files on **GitHub Pages** — its D1 chose a rebuilt-on-schedule
static site precisely to avoid a runtime server. But the athlete already runs the
Kazper cluster (ingress-nginx + cert-manager) and wants **one operational home**:
serve the page from the cluster via the existing Helm chart instead of a second
hosting provider with its own settings, secrets, and Pages enablement dance.

This supersedes that change's Pages topology with an **in-cluster nginx
container**, consciously re-accepting a small property the static design
avoided — public traffic reaching the cluster ingress — in exchange for a single
deploy surface. The mitigations keep the exposure narrow: the public workload is
a **stock nginx serving two baked static files on its own ingress host with no
route to the API**, and the feed **secret still lives only at image-build time**
(build-time bake), so nothing sensitive enters the cluster at all.

## What Changes

- **New multi-stage `Dockerfile` for `apps/public`**: a build stage runs the
  existing `npm run build` (feed fetch → astro build → OG card) with
  `FEED_SECRET` supplied as a **BuildKit build secret** and `FEED_URL` as a build
  arg; the final stage is `nginx:alpine` serving the baked `dist/`. The feed
  secret and the Kazper origin appear in **no layer** of the final image.
- **The `public-site` GitHub Actions workflow retargets** from "deploy to Pages"
  to "build + push the image to GHCR, then roll the in-cluster Deployment onto
  it". Triggers (nightly cron + `workflow_dispatch` + `apps/public/**` push) are
  unchanged; a failed build leaves the running Deployment serving the previous
  image.
- **Helm chart gains an opt-in `publicSite` block** (default off → renders
  nothing, mirroring `backup.enabled`): a `Deployment` (the nginx image) + a
  `Service` + a **dedicated public Ingress host** (TLS via cert-manager) that has
  **no rule routing to the API `Service`**.
- **Supersedes `add-public-race-site` D1/D7** (Pages) and extends D5's secret
  hygiene to the container image. The feed endpoint, the build-time fetch, the
  client-side countdown, the off-season null state, and the OG card are all
  unchanged — only the *serve + deploy* layer moves.
- **Docs**: chart `README` documents the `publicSite` values; `apps/public`
  README and the top-level README topology diagram move from Pages to in-cluster.

## Capabilities

### New Capabilities

_None._ This retargets the existing `public-race-site` serve layer and extends
the `deployment-pipeline` chart.

### Modified Capabilities

- `public-race-site`: the "built statically … no runtime coupling" requirement's
  secret-hygiene clause extends to the **container image layers**, and the
  "scheduled pipeline" requirement retargets from GitHub Pages to a built + pushed
  image rolled onto an in-cluster Deployment. The countdown-freshness,
  null-state, and share-card requirements are untouched.
- `deployment-pipeline`: an ADDED requirement covering the opt-in public-site
  image + Helm workload (Deployment/Service/Ingress on a distinct public host,
  with no path to the API).

## Impact

- **New code:** `apps/public/Dockerfile` (+ a minimal `nginx.conf` if the default
  needs adjusting); rewritten `.github/workflows/public-site.yml`; new
  `deploy/helm/kazper/templates/public-site.yaml`; `ingress.yaml`/`values.yaml`
  additions for the `publicSite` block; chart + app READMEs. No Go, no migration,
  no REST/MCP change, no swag.
- **Ops:** the build `FEED_SECRET`/`FEED_URL` move to the **image build**
  (BuildKit secret + arg); `publicSite.enabled` + `publicSite.host` + the image
  tag go in `values`; a public DNS record points at the cluster ingress. Public
  traffic now lands on the cluster ingress (static nginx only). Supersedes the
  Pages-enablement operator steps from `add-public-race-site`.
- **Cutover:** enable `publicSite`, point DNS at the ingress, verify, then retire
  the Pages deploy path. Rollback = `publicSite.enabled: false` (chart renders
  nothing) and, if needed, re-enable Pages.

### Out of scope (explicit non-goals)

- A GitOps/ArgoCD auto-image-bump controller — the rollout is a workflow step;
  the exact image-tag→rollout mechanism (mutable tag + `rollout restart` vs.
  immutable tag + value bump) is an operator choice the spec leaves open, only
  requiring the Deployment to end up on the freshly-built image and never on a
  failed build.
- The v2 training-derived `progress` number, editorial CMS, and a custom domain
  beyond the ingress host — all unchanged deferrals from `add-public-race-site`.
- Any change to the feed endpoint, its gating, or its projection.
