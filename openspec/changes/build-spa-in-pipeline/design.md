## Context

`apps/web/embed.go` declares `//go:embed all:dist` and `RegisterSPA` reads `index.html` from
it. The committed `apps/web/dist` makes `go build` Node-free, but the artifact is unguarded:
CI runs `go vet` + `go test ./...` + a Docker build, with **no SPA build, no vitest, no
drift check**. `TestSPA_RealEmbeddedBuildServes` asserts the embed serves a real `<div
id="root">` shell *and* a real hashed asset under `dist/assets` — it passes today only because
the real bundle is committed.

Two hard constraints shape any "don't commit dist" design:
1. **`//go:embed all:dist` is a compile-time requirement** — without a `dist/` directory the
   package fails to *compile*, so `go vet`/`go test`/`go build` all break on a fresh checkout.
2. **The real-build test needs a real build** — it requires an asset under `dist/assets`, which
   a stub doesn't have.

## Goals / Non-Goals

**Goals:**
- `apps/web/dist` is no longer committed; the shipped dashboard is always built from source.
- Default Go builds/tests still work with no Node toolchain and no committed build artifact.
- CI builds + tests the SPA and exercises the real embedded build.

**Non-Goals:**
- Touching the `docs/` swagger precedent (that stays committed).
- Any runtime/API/auth/data change.
- Optimizing away the double SPA build (CI builds it for tests; the image builds it again) —
  acceptable; can be revisited.

## Decisions

### D1 — Build-tag split: committed stub by default, real dist under `-tags webembed`

`embed.go` splits in two files in package `webapp`:

```
embed_stub.go   //go:build !webembed   →  //go:embed all:stub   (committed shell)
embed_dist.go   //go:build  webembed   →  //go:embed all:dist   (gitignored, built)
```

Both expose the same `DistFS()`. Default builds embed a tiny **hand-written** `apps/web/stub/
index.html` (a `<div id="root">` shell with a "run `task web:build`" note) — so `go build`/`go
test`/`go vet` compile with zero Node and nothing generated is committed. Release/image builds
pass `-tags webembed` and embed the real `apps/web/dist` produced by `npm run build`.

**Why a committed stub doesn't violate "don't commit dist":** the stub is a 5-line
hand-authored file, not a build output — it never churns, never drifts, and isn't the minified
bundle the user objected to. It exists solely to satisfy the `go:embed` compile constraint.

**Alternative considered — B: no stub, always build.** Gitignore `dist` entirely and make
`task build`/`task test` depend on `web:build`, with CI running `npm run build` before every Go
step. Rejected as the default because a raw `go build ./cmd/kazper` (no Task, no Node) then
fails to *compile* — a worse first-run experience than embedding a stub. D1 keeps Go ergonomics
while still never committing the bundle.

### D2 — `TestSPA_RealEmbeddedBuildServes` moves behind `//go:build webembed`

The real-build test asserts a hashed asset exists, so it is only meaningful against a real
build. Tag it `webembed`; it's skipped in the default `go test ./...` (which now embeds the
stub) and runs in the dedicated CI step that builds the SPA and invokes `go test -tags webembed
./internal/httpserver/...`. The fixture-based SPA serving tests stay untagged (they use an
in-memory `fs.FS`, not the embed).

### D3 — Multi-stage Dockerfile

Prepend a Node stage; the Go stage copies the built dist and tags the build:

```dockerfile
FROM node:22-alpine AS web
WORKDIR /web
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build                 # → /web/dist

FROM golang:1.26-alpine AS build
...
COPY . .
COPY --from=web /web/dist ./apps/web/dist
RUN ... go build -trimpath -tags webembed -ldflags=... ./cmd/kazper
```

`.dockerignore` keeps `apps/web` source (the Node stage needs it), drops `apps/web/node_modules`
and the now-gitignored `dist`. (This supersedes the interim `.dockerignore` fix that re-included
the committed dist.)

### D4 — CI builds and tests the SPA

`pr.yml` and `main.yml` gain a Node setup + `npm ci && npm test && npm run build`, then `go
test -tags webembed ./internal/httpserver/...`. `pr.yml` stops path-ignoring `apps/web/**` so a
web-only PR is actually validated. The image build (D3) self-contains the SPA build, so the
published image always ships a fresh dashboard regardless of the test job.

### D5 — Taskfile ergonomics

`task build` depends on `task web:build` so a locally-built binary embeds the real dashboard
(`task build` already runs `go build`; add `-tags webembed` there and the dep). `task dev`
(source-run via `go run`) keeps using the stub unless `web:build` was run — documented, since
dev mode usually drives the Vite dev server anyway.

## Risks / Trade-offs

- **Raw `go build` ships the stub.** Mitigation: `task build` does the right thing; the image
  path always embeds the real build; the stub page links to `task web:build`.
- **Double SPA build (CI test job + image).** Accepted; small, cache-friendly.
- **Contributor confusion over the tag.** Mitigation: a comment in both embed files + a
  `RUN_LOCAL.md` note.

## Migration Plan

`git rm --cached -r apps/web/dist`; add to `.gitignore`; add `apps/web/stub/index.html`; split
embed files; tag the real-build test; rewrite Dockerfile + `.dockerignore`; update CI +
Taskfile + docs. Verify: `go test ./...` (stub) green; `go test -tags webembed
./internal/httpserver/...` green after `npm run build`; `docker build .` succeeds and the image
serves the real dashboard.

## Open Questions

- Pin the Node base/version to the project's local Node (`node:22-alpine` assumed) — confirm.
- Should `task dev` auto-run `web:build` once for parity, or stay stub + Vite-dev? (Leaning
  stay — dev uses the Vite server.)
