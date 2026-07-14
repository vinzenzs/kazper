# Tasks — host the public race site in-cluster

## 1. Container image (`apps/public`)

- [x] 1.1 `apps/public/Dockerfile`: multi-stage — `node:22-alpine` build stage
  (`npm ci`, copy source, `RUN --mount=type=secret,id=feed_secret` exporting
  `FEED_SECRET` from `/run/secrets/feed_secret` with `FEED_URL`/`SITE_URL`/
  `BASE_PATH` build args, `npm run build`) → `nginx:alpine` stage copying
  `/app/dist` to `/usr/share/nginx/html`. Add `apps/public/.dockerignore`
  (`node_modules`, `dist`, `.astro`, `src/generated`).
- [x] 1.2 Minimal `nginx.conf` only if the default needs adjusting (root serve,
  `try_files`, sensible cache headers for `index.html` vs. `og.png`); otherwise
  rely on the stock config and note that.
- [x] 1.3 Local build verify against a mock feed: `docker build` with the
  BuildKit secret + `FEED_URL` arg produces an image whose layers and build
  history contain **neither** the secret value **nor** `FEED_URL`
  (`docker history --no-trunc` + a layer grep); `docker run` serves `index.html`
  (race + prerendered countdown) and `og.png`.

## 2. CI workflow (retarget `public-site.yml`)

- [x] 2.1 Rewrite `.github/workflows/public-site.yml`: build + push the image to
  GHCR via `docker/build-push-action` with the feed secret passed as a BuildKit
  `secrets:` entry and `FEED_URL`/`SITE_URL`/`BASE_PATH` as build args (from
  Actions `vars`/`secrets`). Keep the triggers (nightly cron + `workflow_dispatch`
  + `push` filtered to `apps/public/**` and the workflow file).
- [x] 2.2 After a successful push, roll the in-cluster Deployment onto the new
  image (the chosen tag/rollout mechanism, documented in the workflow). A failed
  build MUST NOT push or roll — the previous image keeps serving.
- [x] 2.3 Remove the GitHub Pages steps + `pages`/`id-token` permissions +
  `github-pages` environment; drop the Pages-specific config.

## 3. Helm chart (`deploy/helm/kazper`)

- [x] 3.1 `values.yaml`: add the `publicSite` block (`enabled: false`, `image`
  {repository, tag, pullPolicy}, `replicas`, `host`, `ingress`
  {className, tls, clusterIssuer}, `resources`), with comments.
- [x] 3.2 `templates/public-site.yaml`: gated on `.Values.publicSite.enabled` — a
  `Deployment` (nginx image, `publicSite.replicas`, liveness/readiness on `/`
  port 80, **no `FEED_SECRET` env or secret mount**) + a `Service` (port 80),
  using the chart's `_helpers.tpl` labels/naming (mirror `garmin-bridge.yaml`).
- [x] 3.3 A dedicated public-site `Ingress` for `publicSite.host` (TLS via the
  chart's cert-manager/ingress-nginx assumptions), backend = the public-site
  Service only. Keep it separate from the API `ingress.yaml` so no rule ever
  targets the API Service.
- [x] 3.4 Confirm the gating: with `publicSite.enabled: false` the templates emit
  **zero** public-site objects (the `backup.enabled` precedent) and the API
  objects are byte-identical to before.

## 4. Docs

- [x] 4.1 Chart `README`: document the `publicSite` values, the install/upgrade
  path, and the cutover (point the public DNS record at the cluster ingress;
  supply `FEED_SECRET`/`FEED_URL` to the image build, not the pod).
- [x] 4.2 `apps/public/README.md` + the top-level `README` topology diagram: move
  from "GitHub Pages" to "in-cluster nginx container behind the chart ingress"
  (CI builds+pushes the image, rolls the Deployment; secret stays build-time).

## 5. Verification

- [x] 5.1 `helm template` with default values → no public-site Deployment/Service/
  Ingress; API Deployment/Service/Ingress unchanged.
- [x] 5.2 `helm template` with `publicSite.enabled=true` + `host` + `image` →
  renders the three objects; assert the public Ingress's only backend is the
  public-site Service (no API rule), the pod spec has no `FEED_SECRET`, and TLS
  references the configured issuer. `helm lint` clean.
- [x] 5.3 Local `docker build` against a mock feed (task 1.3) — race render +
  image-layer/history secret+`FEED_URL` hygiene (both absent) + container serves
  `index.html`/`og.png`; repeat once with a `{race:null}` mock for the off-season
  image.
- [ ] 5.4 Operator cutover (**follow-up — not executable from the dev _**(manual — needs cluster/DNS/secrets; not runnable from this environment)**_
  environment**): set `publicSite.*` + the image tag, add `FEED_SECRET`/`FEED_URL`
  to the CI/build environment, point DNS at the ingress, verify the live page +
  TLS + OG card resolve, then retire the Pages deploy path.
