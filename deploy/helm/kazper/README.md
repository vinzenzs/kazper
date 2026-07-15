# kazper Helm chart

Single-replica deployment of the [kazper](https://github.com/vinzenzs/kazper)
REST backend. Ships a `Deployment`, `Service`, `ConfigMap`, optional
`Ingress`, optional `Secret`, and a `ServiceAccount`. Expects an
externally provisioned Postgres reachable via `DATABASE_URL` — the chart
does **not** include a Postgres subchart.

## Prerequisites

- Kubernetes 1.27+
- A reachable Postgres instance (managed or self-hosted)
- For Ingress + TLS (optional): `ingress-nginx` + `cert-manager` already
  installed in the cluster

## Required values

When the chart owns the Secret (default), three values MUST be set or
`helm install` fails with a clear error:

| Value | What it is |
|---|---|
| `secrets.databaseUrl` | Postgres connection string |
| `secrets.mobileApiToken` | Bearer token for the mobile companion app |
| `secrets.agentApiToken` | Bearer token for the LLM agent (must differ from `mobileApiToken`) |

`secrets.anthropicApiKey` is optional. When empty, the `POST
/meals/from_photo` endpoint returns `503 vision_unavailable` (existing
behaviour); the rest of the API works normally.

`secrets.feedSecret` is optional. When set, the backend enables the public
race feed (`GET /public/race-feed`) and validates the `X-Feed-Key` request
header against it (constant-time); when empty the endpoint returns `503
feed_disabled`. This is the API server's copy of the secret — the optional
`publicSite` pod deliberately holds **no** feed secret (it bakes the feed into
static files at image-build time), so set the same value as the build's
`FEED_SECRET` when you run both.

To manage the Secret outside the chart instead (e.g., applied via
`kubectl apply` from a SOPS-encrypted file), pass `--set
existingSecret=my-secret-name`. The Secret must contain these keys:
`DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN` (and optionally
`ANTHROPIC_API_KEY`, `FEED_SECRET` when the public race feed is enabled, plus
`FCM_SERVICE_ACCOUNT_JSON` when push is enabled).

## Push notifications (opt-in)

Android push (FCM HTTP v1 — used for the Garmin relogin notification) is
off by default. Enabling it requires **both** values together:

| Value | What it is |
|---|---|
| `config.fcmProjectId` | Firebase project id (non-secret, lands in the ConfigMap) |
| `secrets.fcmServiceAccountJson` | Google service-account credential — inline JSON or a path to a mounted credential file |

When `config.fcmProjectId` is set, `secrets.fcmServiceAccountJson` is
**required** — `helm install` fails with a clear error otherwise (the
backend validates the credential at startup). With both empty, the push
surface is inert: it returns `503 push_disabled` and the relogin
notification is a no-op.

Because the credential is multi-line JSON, prefer `--set-file`:

```bash
helm upgrade --install kazper oci://ghcr.io/vinzenzs/charts/kazper \
    --version v0.1.0 --namespace kazper \
    --set config.fcmProjectId=my-firebase-project \
    --set-file secrets.fcmServiceAccountJson=service-account.json \
    # ...required token values...
```

When using `existingSecret`, put `FCM_SERVICE_ACCOUNT_JSON` in that Secret
and still set `config.fcmProjectId` in values.

## Coach dashboard (opt-in)

The training dashboard is a browser SPA embedded in the binary and served
same-origin at `/` (the API stays under `/api/v1`). It is off by default.
Enabling it requires **both** values together:

| Value | What it is |
|---|---|
| `secrets.webUser` | HTTP Basic username for the dashboard (`client_id=web`, full access) |
| `secrets.webPassword` | HTTP Basic password |

Set one without the other and `helm install` fails with a clear error (the
backend recognizes the `web` identity only when both are present). With both
empty the dashboard shell still loads but its API calls are unauthenticated.

```bash
helm upgrade --install kazper oci://ghcr.io/vinzenzs/charts/kazper \
    --version v0.1.0 --namespace kazper \
    --set secrets.webUser=coach \
    --set secrets.webPassword="$(openssl rand -hex 24)" \
    # ...required token values...
```

> **Transport expectation.** HTTP Basic auth transmits the credential as
> base64 (not encryption) on every request. Serve the dashboard only over an
> encrypted transport — terminate TLS at the Ingress (see `ingress.tls` below)
> or reach it over a Tailscale tailnet. Never expose it on a bare HTTP
> endpoint. When using `existingSecret`, put `WEB_USER` / `WEB_PASSWORD` in
> that Secret.

## Public race site (opt-in)

The public "road to race" page (`apps/public`) can be served in-cluster as a
static `nginx` container, gated on `publicSite.enabled` (default **off** → the
chart renders no public-site objects). It is deliberately **isolated**: its own
public Ingress host with **no route to the API**, and the pod holds **no feed
secret** (the page is baked from the race feed at image-build time — the secret
is a BuildKit build secret, never in the image or the running pod).

| Value | What it is |
|---|---|
| `publicSite.enabled` | Render the public-site Deployment + Service + Ingress |
| `publicSite.host` | Public hostname (e.g. `race.example.com`); **required** when enabled |
| `publicSite.image.repository` / `.tag` | The image the `public-site` CI workflow publishes (default `:latest`, `pullPolicy: Always`) |
| `publicSite.ingress.className` / `.tls.enabled` / `.tls.issuer` | Ingress class + cert-manager TLS (same assumptions as the API `ingress`) |

```bash
helm upgrade --install kazper oci://ghcr.io/vinzenzs/charts/kazper \
    --version v0.1.0 --namespace kazper \
    --set publicSite.enabled=true \
    --set publicSite.host=race.example.com \
    # ...required token values...
```

Then point a public DNS record for `publicSite.host` at the cluster ingress. The
image is (re)built + pushed and the Deployment rolled by
`.github/workflows/public-site.yml` (nightly + on-demand + on `apps/public/**`
push); supply `FEED_SECRET` (Actions secret) and `FEED_URL` (Actions variable)
to the build. A failed build never rolls — the previous image keeps serving, and
the countdown recomputes client-side, so the page degrades to stale, never
broken.

> The public workload is a static file server on its own host; hardening beyond
> `runAsNonRoot` (rate-limit annotations, a WAF) is a follow-up if abuse ever
> appears.

## Install a tagged release (OCI from GHCR)

```bash
helm upgrade --install kazper \
    oci://ghcr.io/vinzenzs/charts/kazper \
    --version v0.1.0 \
    --namespace kazper --create-namespace \
    --set secrets.databaseUrl='postgres://nutrition:...@db.internal:5432/nutrition?sslmode=disable' \
    --set secrets.mobileApiToken='<openssl rand -hex 32>' \
    --set secrets.agentApiToken='<openssl rand -hex 32>'
```

For sensitive values, prefer a private values file over `--set` (which
ends up in shell history):

```bash
# private-values.yaml — keep out of git, e.g. SOPS-encrypted or in a secrets manager
secrets:
  databaseUrl: postgres://...
  mobileApiToken: ...
  agentApiToken: ...
  anthropicApiKey: sk-ant-...   # optional
ingress:
  enabled: true
  host: nutrition.example.com
  tls:
    enabled: true
    issuer: letsencrypt-prod
```

```bash
helm upgrade --install kazper \
    oci://ghcr.io/vinzenzs/charts/kazper \
    --version v0.1.0 \
    --namespace kazper --create-namespace \
    -f private-values.yaml
```

## Install from the repo (untagged / development)

```bash
helm upgrade --install kazper \
    ./deploy/helm/kazper/ \
    --set image.tag=main \
    --set secrets.databaseUrl=... \
    --set secrets.mobileApiToken=... \
    --set secrets.agentApiToken=...
```

## Upgrades

Same command — Helm idempotently reconciles. The Deployment uses
`strategy.type: Recreate` (one replica, single-process migrations), so
expect brief downtime during each upgrade.

The chart sets a `checksum/config` (and `checksum/secret` when
chart-managed) pod annotation, so config-only changes trigger a pod
restart automatically.

## Rollback

```bash
helm history kazper --namespace kazper
helm rollback kazper <REVISION> --namespace kazper
```

Rolling back the chart restores the prior image + values. If the
rollback target has a different schema_migrations head, the binary will
re-apply forward to whatever the embedded migrations claim (so rolling
the binary back across a migration boundary requires a separate
`kazper migrate -version <N> down` step against the database
beforehand).

## Smoke-test the install

```bash
kubectl -n kazper rollout status deploy/kazper
kubectl -n kazper port-forward svc/kazper 8080:80 &
curl -s http://localhost:8080/healthz   # {"status":"ok"}
curl -s http://localhost:8080/readyz    # {"status":"ok"}
```

`kazper version` inside the pod reports the embedded build
identity:

```bash
kubectl -n kazper exec deploy/kazper -- /app/kazper version
# kazper version=v0.1.0 commit=<sha> date=unknown
```

## Probes

- `livenessProbe` → `/healthz` — unconditional 200; only fails if the
  process is wedged.
- `readinessProbe` → `/readyz` — pings Postgres; flips the pod out of
  the Service's endpoints within ~30 s of a Postgres outage. Desired —
  but means a Postgres flap is visible to clients as a 503 from the
  edge.

## Resources

The defaults (`50m CPU / 64Mi memory` requests, `500m / 256Mi` limits)
fit a personal cluster; bump them if you see throttling or OOM in
`kubectl top`.

## Tearing down

```bash
helm uninstall kazper --namespace kazper
kubectl delete namespace kazper   # if you created it for this chart
```

The chart does NOT touch your Postgres — data survives uninstall.

## Out of scope

- Postgres provisioning (run separately, point `databaseUrl` at it).
- Multi-replica, autoscaling, PodDisruptionBudget — single user, single
  replica.
- Image signing, SBOM, multi-arch builds — `linux/amd64` only.
- Multi-environment overlays (`values-staging.yaml` etc.) — one
  install, one values file.
