## Why

The coach-dashboard SPA build output (`apps/web/dist`) is **committed** to git and embedded via
`//go:embed all:dist`, mirroring the `docs/` swagger precedent. That keeps `go build`
Node-free, but it puts a minified, content-hashed build artifact under version control with
**no freshness guarantee**: nothing in CI rebuilds it or fails when the committed `dist` drifts
from `apps/web/src`. A stale dashboard can ship silently, and the bundle churns the diff on
every UI change. (It also just broke the image build — `.dockerignore` excluded `apps/`, so the
embedded package didn't resolve in the Docker context.)

This change stops committing the build artifact: `apps/web/dist` becomes gitignored, the
**Dockerfile builds the SPA in a Node stage** before the Go stage embeds it, and **CI builds +
tests the SPA** so the shipped dashboard is always freshly built from source. The `docs/`
swagger precedent is unaffected — this is specifically about the SPA bundle.

## What Changes

- **Stop committing `apps/web/dist`** — add it to `.gitignore` and `git rm --cached` the
  current bundle.
- **Build-tag the embed** so default Go builds don't require a built `dist`:
  - default (no tag) embeds a tiny **committed hand-written stub** shell — `go build` / `go
    test` / `go vet` stay Node-free and the package always compiles;
  - `-tags webembed` embeds the real `apps/web/dist`, used only by the release/image build
    after `npm run build`.
- **Multi-stage Dockerfile** — a `node:22-alpine` stage runs `npm ci && npm run build`; the Go
  stage copies the built `dist` in and compiles with `-tags webembed`. Runtime stage unchanged
  (distroless, binary-only).
- **CI gains the SPA** — a step that runs `npm ci`, `npm test` (the 34 vitest tests), and
  `npm run build`, then a tagged Go check (`go test -tags webembed ./internal/httpserver/...`)
  so `TestSPA_RealEmbeddedBuildServes` runs against the freshly-built bundle. `pr.yml` stops
  ignoring `apps/web/**`.
- **`.dockerignore`** keeps `apps/web` *source* in context (the Node stage needs it) and
  excludes `node_modules` + the gitignored `dist`.
- **Taskfile** — `task build` depends on `task web:build` so a local binary embeds a real
  dashboard; document that a raw `go build -tags webembed` needs `dist` present.

## Capabilities

### New Capabilities
<!-- none — this refines the existing deployment-pipeline capability -->

### Modified Capabilities

- `deployment-pipeline`: the container image SHALL build the web SPA in a Node stage and embed
  it (the build artifact is no longer committed); the PR and main workflows SHALL build and
  test the SPA so the embedded dashboard is always built from source.

## Impact

- **Build** (`Dockerfile`, `.dockerignore`, `.gitignore`, `Taskfile.yml`): multi-stage image;
  `apps/web/dist` un-tracked + ignored; `task build` gains a `web:build` dep.
- **Embed** (`apps/web/`): split `embed.go` into a stub (`//go:build !webembed`) over a
  committed `apps/web/stub/` shell and a real (`//go:build webembed`) `//go:embed all:dist`.
- **Test** (`internal/httpserver/spa_test.go`): `TestSPA_RealEmbeddedBuildServes` moves behind
  `//go:build webembed` (it asserts a real hashed asset, which only exists in a real build).
- **CI** (`.github/workflows/pr.yml`, `main.yml`): add Node setup + `npm ci/test/build` + a
  tagged Go check; `pr.yml` no longer path-ignores `apps/web/**`.
- **Trade-off**: a raw `go build` of the binary now embeds the *stub* dashboard unless built
  `-tags webembed` (which needs `dist`). The real dashboard always ships via the image/CI path.
  This is the deliberate inverse of the committed-dist trade — clean history + no drift, at the
  cost of Node in the release build.
- **No runtime, API, auth, or data change.**
