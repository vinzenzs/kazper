## 1. Stop committing the build artifact

- [x] 1.1 `git rm --cached -r apps/web/dist`; add `apps/web/dist/` to `.gitignore`.
- [x] 1.2 Add a hand-written `apps/web/stub/index.html` — a minimal `<div id="root">` shell with a "run `task web:build` for the real dashboard" note. Commit it (it is source, not a build output).

## 2. Build-tag the embed

- [x] 2.1 Split `apps/web/embed.go` into `embed_stub.go` (`//go:build !webembed`, `//go:embed all:stub`) and `embed_dist.go` (`//go:build webembed`, `//go:embed all:dist`). Both expose the same `DistFS()`/package API.
- [x] 2.2 Confirm default `go build`/`go vet`/`go test ./...` compile with no `dist/` present (stub embedded).

## 3. Gate the real-build test

- [x] 3.1 `internal/httpserver/spa_test.go`: move `TestSPA_RealEmbeddedBuildServes` behind `//go:build webembed` (it asserts a real hashed asset). Keep the fixture-fs serving tests untagged.
- [x] 3.2 Verify `go test -tags webembed ./internal/httpserver/...` passes after `npm run build`, and the default `go test ./...` skips it cleanly.

## 4. Multi-stage Dockerfile + dockerignore

- [x] 4.1 `Dockerfile`: add a `node:22-alpine` stage running `npm ci && npm run build`; the Go stage `COPY --from=web /web/dist ./apps/web/dist` and builds with `-tags webembed`. Runtime stage unchanged.
- [x] 4.2 `.dockerignore`: keep `apps/web` source in context, exclude `apps/web/node_modules` and `apps/web/dist`. Supersede the interim re-include fix.
- [x] 4.3 `docker build .` succeeds; run the image and confirm `GET /` (with `WEB_USER`/`WEB_PASSWORD`) serves the real dashboard shell + a hashed asset.

## 5. CI

- [x] 5.1 `pr.yml` + `main.yml`: add Node setup + `npm ci && npm test && npm run build` (working-dir `apps/web`), then `go test -tags webembed ./internal/httpserver/...`.
- [x] 5.2 `pr.yml`: stop path-ignoring `apps/web/**` so web-only PRs are validated.
- [x] 5.3 Confirm the image build step still passes (it self-contains the SPA build via the Dockerfile).

## 6. Taskfile + docs

- [x] 6.1 `Taskfile.yml`: `task build` depends on `task web:build` and builds `-tags webembed`; document the stub-vs-real behavior.
- [x] 6.2 `RUN_LOCAL.md` / `README.md`: note that `apps/web/dist` is built, not committed; the binary embeds the stub unless built via `task build` / the image.

## 7. Verification

- [x] 7.1 `go test ./...` green (stub embedded); `go vet ./...` clean.
- [x] 7.2 `npm test` (vitest, 34) green; `npm run build` clean.
- [x] 7.3 `go test -tags webembed ./internal/httpserver/...` green against a fresh `dist`.
- [x] 7.4 `docker build .` green + live image smoke (real dashboard served).
- [x] 7.5 Confirm `git status` shows no `apps/web/dist` tracked and the bundle no longer appears in diffs.
