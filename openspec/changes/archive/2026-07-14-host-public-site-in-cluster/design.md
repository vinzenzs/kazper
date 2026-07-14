# Design — host the public race site in-cluster

## Context

`add-public-race-site` built `apps/public` (Astro static output) and deployed it
to GitHub Pages. Its central decision (D1) was "a static site rebuilt on a
schedule replaces a running server — at serve time there is no server, no secret,
and no path from public traffic to Kazper," and it explicitly rejected an edge
worker because that "reintroduces a runtime secret and a serve-time dependency."

This change keeps the **artifact** (the same baked static `dist/`) but moves the
**serve layer** into the athlete's existing k8s cluster. That reopens exactly one
of D1's properties — "no public traffic on the cluster" — and re-decides it. The
build-time-bake property (no runtime secret) is preserved.

Repo precedent: the Helm chart (`deploy/helm/kazper`) already renders opt-in
sub-workloads (`backup.enabled` → CronJob + PVC; `garminBridge` → a second
Deployment) and an opt-in `ingress` assuming ingress-nginx + cert-manager. The
public site follows the same shape.

## Goals / Non-Goals

**Goals:**
- Serve the public page from the cluster via the Helm chart — one deploy surface.
- Keep the feed secret out of the cluster entirely (build-time bake, BuildKit
  secret); the image contains only static files.
- Keep the served page correct day-to-day even when the image is stale (the
  client-side countdown is unchanged), so the nightly rebuild stays a safety net.
- Narrow the newly-accepted exposure: static nginx, its own ingress host, no
  route to the API.

**Non-Goals:**
- A CMS or any runtime application logic in the public workload (it is nginx +
  two files).
- GitOps controller specifics; the v2 `progress` number; a custom domain beyond
  the ingress host.
- Any backend / feed change.

## Decisions

### D1 — In-cluster nginx container supersedes GitHub Pages (re-decides add-public-race-site D1)

Serve the baked `dist/` from a `nginx:alpine` container behind the chart's
ingress instead of GitHub Pages. This **gives up** add-public-race-site's "no
public traffic on the cluster" property; accepted because:

- **One operational home.** The cluster, ingress-nginx, and cert-manager already
  exist for Kazper; a second hosting provider (Pages) is a separate settings +
  secrets surface to maintain. The athlete asked for the single surface.
- **The exposure is a static file server, not Kazper.** The public workload
  serves two prebuilt files and has **no reverse-proxy rule to the API Service**
  — it is a distinct Ingress host in the same chart, not a path on the API host.
  An attacker reaching it reaches nginx, not `/api/v1` or `/public/race-feed`.
- **The secret never enters the cluster** (D2): build-time bake means the running
  pod holds no `FEED_SECRET` and makes no call to Kazper.

Trade-off vs. Pages: the operator now owns an nginx image to patch and an ingress
to expose. Mitigation: it is stock `nginx:alpine` (tracked like any base image)
serving static content; optional ingress rate-limiting annotations are available
if abuse ever appears.

### D2 — Build-time bake in a multi-stage image; the secret is a BuildKit secret

A two-stage `Dockerfile`:

```
# build stage
FROM node:22-alpine AS build
WORKDIR /app
COPY apps/public/package*.json ./
RUN npm ci
COPY apps/public/ ./
ARG FEED_URL
RUN --mount=type=secret,id=feed_secret \
    FEED_SECRET="$(cat /run/secrets/feed_secret)" FEED_URL="$FEED_URL" \
    SITE_URL="$SITE_URL" BASE_PATH="/" npm run build

# serve stage
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
```

The `--mount=type=secret` keeps `FEED_SECRET` out of every layer and the build
history; `FEED_URL` is a build arg (not sensitive, but also not needed at
runtime). The final image is `nginx:alpine` + the baked `dist/` — **no secret, no
`FEED_URL`, no node toolchain**. `SITE_URL`/`BASE_PATH` become build args set to
the public host and `/` (in-cluster serves at the ingress root, unlike Pages'
project sub-path).

### D3 — Nightly refresh rebuilds the image and rolls the Deployment

Because the content is **baked into the image**, refreshing the prerendered
countdown + OG card requires rebuilding the image (re-running the feed fetch at
build) — a k8s CronJob cannot rebuild baked content. So the nightly refresh stays
in **CI**: the `public-site` workflow builds + pushes a new image and triggers a
rollout of the Deployment. The client-side countdown (unchanged from
add-public-race-site) keeps the *served page* correct daily regardless, so a
missed rebuild only staleness the OG card and the no-JS fallback — the rebuild is
a safety net, not load-bearing. **A failed build never rolls**; the Deployment
keeps serving the last good image.

The image-tag → rollout mechanism is left to the operator (mutable tag +
`kubectl rollout restart` / imagePullPolicy Always, or immutable tag + a Helm
value bump / GitOps). The spec requires only the two invariants: the Deployment
ends up on the freshly-built image, and a failed build changes nothing.

### D4 — Opt-in `publicSite` Helm block, isolated from the API

New chart values (default off, so an existing install renders **zero** new
objects — the `backup.enabled` precedent):

```yaml
publicSite:
  enabled: false
  image:
    repository: ghcr.io/vinzenzs/kazper-public-site
    tag: ""            # set by CI / release
    pullPolicy: IfNotPresent
  replicas: 1
  host: ""             # e.g. race.example.com
  ingress:
    className: nginx
    tls: true
    clusterIssuer: letsencrypt-prod
  resources: {}        # tiny; static nginx
```

Renders a `Deployment` (nginx image, single replica, `/` liveness/readiness on
port 80), a `Service`, and a **dedicated Ingress** for `publicSite.host` whose
only backend is the public-site Service — **never the API Service**. Reuses the
chart's cert-manager/ingress-nginx assumptions (the existing `ingress`
requirement). The API's own ingress and Service are untouched.

### D5 — Cutover and the archived Pages design

This design **supersedes add-public-race-site's D1 (Pages) and D7 (Pages
workflow)** going forward; the archived design is not edited (archives are
history). The workflow file is rewritten in place (image build+push+roll instead
of Pages deploy). Cutover: enable `publicSite`, point the public DNS record at
the cluster ingress, verify the page + TLS + OG card, then the Pages path is
retired (nothing references it). Rollback: `publicSite.enabled: false`.

## Risks / Trade-offs

- **[Risk] Public traffic now reaches the cluster ingress.** → Mitigation (D1):
  static nginx, its own host, no API route; optional rate-limit annotations. The
  attack surface is a file server, not the app.
- **[Risk] OG card / no-JS fallback go stale if the nightly rebuild fails.** →
  Accepted (unchanged from add-public-race-site): the client-side countdown keeps
  the live page correct; Actions failure emails surface a stuck rebuild.
- **[Risk] The rollout mechanism is deployment-environment-specific.** →
  Mitigation (D3): the spec fixes the invariants (fresh image, never-on-failure)
  and leaves the mechanism to the operator; the workflow ships a sensible default.
- **[Trade-off] Baked content needs an image rebuild to refresh, vs. Pages'
  file-level deploy.** → Accepted: the rebuild is cheap (npm ci + astro build of a
  one-page site) and the client-side countdown makes daily freshness non-critical.

## Migration Plan

Purely additive to the chart (opt-in, default off) plus a rewritten workflow and
a new Dockerfile. No backend deploy, no migration. Enable by setting
`publicSite.*`, adding `FEED_SECRET`/`FEED_URL` to the CI/build environment, and
pointing DNS at the ingress. Roll back by disabling `publicSite`.

## Open Questions

- **Image-tag/rollout mechanism** — mutable `:public-main` + `rollout restart`,
  or immutable `:public-<sha>` + a values bump? Left to the operator's GitOps
  setup; the workflow defaults to one and documents it.
- **Rate limiting / WAF on the public ingress** — deferred; add annotations only
  if real abuse appears (a static file server is a low-value target).
- **Whether to keep a Pages mirror as a fallback** — no; one surface is the whole
  point. The `dist/` artifact stays portable if that ever changes.
