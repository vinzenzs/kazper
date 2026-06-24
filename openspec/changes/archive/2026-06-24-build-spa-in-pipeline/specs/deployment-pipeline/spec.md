## MODIFIED Requirements

### Requirement: The container image is a multi-stage, distroless, statically-linked build

The system SHALL ship as a single container image built from a Dockerfile at the repo root using a multi-stage build. A Node stage SHALL build the web dashboard (`apps/web`) with `npm ci && npm run build`; the Go build stage SHALL copy that built `dist` in and compile the binary with `CGO_ENABLED=0`, `-trimpath`, and `-tags webembed` (which embeds the real SPA build); the runtime stage uses a distroless base image and contains the binary as its only file. Migrations, Swagger docs, and the freshly-built SPA are embedded in the binary at compile time (via `embed.FS`), so the runtime image SHALL NOT need to copy any extra files alongside. The SPA build artifact (`apps/web/dist`) SHALL NOT be committed to the repository — it is gitignored and produced by the build.

#### Scenario: Image runs as a non-root user

- **WHEN** the image is inspected via `docker inspect <image>`
- **THEN** the configured user is `nonroot` (UID 65532)
- **AND** the entrypoint is the embedded `kazper` binary
- **AND** the working directory is `/app`

#### Scenario: Image is statically linked and shell-free

- **WHEN** the image is run with `docker run --rm <image> /bin/sh`
- **THEN** the run fails with "no such file" — the runtime stage has no shell

#### Scenario: The image embeds a freshly-built dashboard

- **WHEN** the image is built from a checkout with no committed `apps/web/dist`
- **THEN** the Node stage builds the SPA and the Go stage embeds it under `-tags webembed`
- **AND** `GET /` (with the `web` Basic credential) serves the real dashboard shell plus a content-hashed asset

#### Scenario: Migrations are embedded, not bind-mounted

- **WHEN** the container starts with `MIGRATE_ON_START=true` and a fresh database
- **THEN** the binary applies all embedded migrations from `internal/store/migrations/` without needing any volume mount
- **AND** the `kazper migrate` subcommand applies migrations against an arbitrary `DATABASE_URL` using the same embedded files

#### Scenario: Image size stays under 30 MB compressed

- **WHEN** the release image is pulled from GHCR
- **THEN** the compressed image size SHALL be under 30 MB

### Requirement: PR workflow validates vet, test, and Docker build without publishing

The system SHALL include `.github/workflows/pr.yml` that runs on `pull_request` events. The workflow SHALL build and test the web dashboard (`npm ci && npm test && npm run build` in `apps/web`), run `go vet ./...`, `go test ./...` (with testcontainers booting its own Postgres), a tagged real-embed check (`go test -tags webembed ./internal/httpserver/...` against the freshly-built `dist`), and `docker buildx build` without `--push`. The workflow SHALL NOT path-ignore `apps/web/**` (a dashboard-only change must be validated). The workflow SHALL fail the PR if any step fails.

#### Scenario: A PR with a failing test blocks merge

- **WHEN** a PR is opened with a change that breaks `go test`
- **THEN** the `pr.yml` workflow run reports failure
- **AND** the failure is associated with the PR's required-checks status

#### Scenario: A PR that breaks the dashboard build or tests blocks merge

- **WHEN** a PR changes `apps/web` such that `npm test` or `npm run build` fails
- **THEN** the workflow run reports failure
- **AND** no image is published

#### Scenario: A PR with a broken Dockerfile blocks merge

- **WHEN** a PR is opened with a change that makes `docker buildx build` fail
- **THEN** the workflow run reports failure
- **AND** no image is pushed to GHCR

#### Scenario: PR also smoke-tests the Helm chart

- **WHEN** the workflow runs
- **THEN** `helm template deploy/helm/kazper/ --debug` is executed
- **AND** the workflow fails if templating produces an error
