# syntax=docker/dockerfile:1.7
#
# Multi-stage build:
#   1. golang:1.26-alpine compiles a statically-linked kazper binary
#      with version + commit injected via -ldflags. Migrations and Swagger
#      docs are embedded via embed.FS in the Go sources, so the runtime
#      stage only needs the binary.
#   2. distroless/static-debian12:nonroot runs as UID 65532. No shell, no
#      package manager — the runtime image's only artefact is the binary
#      at /app/kazper.
#
# Build args (passed by CI in release.yml / main.yml; defaulted for local builds):
#   VERSION  — e.g. "v1.2.3", "main-abc1234"; surfaces via `kazper version`
#   COMMIT   — full git SHA; surfaces via `kazper version`

ARG VERSION=dev
ARG COMMIT=unknown

# Stage 1: build the coach-dashboard SPA. apps/web/dist is NOT committed (per
# build-spa-in-pipeline) — it's produced here and embedded into the binary under
# `-tags webembed` in the Go stage. Runs on the build platform (TS→JS is arch-
# independent), so no QEMU cost on cross-builds.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
# Cache npm install independently of source changes.
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build   # → /web/dist

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG VERSION
ARG COMMIT
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Overlay the freshly-built SPA so `//go:embed all:dist` (under -tags webembed)
# finds it. dist is gitignored, so it isn't in the build context otherwise.
COPY --from=web /web/dist ./apps/web/dist

# CGO_ENABLED=0 + -trimpath give us a deterministic, statically-linked
# binary suitable for distroless/static. -s -w strip symbol + DWARF tables;
# combined they save ~25% of the binary size. -tags webembed embeds the real
# dashboard build (default builds embed the placeholder stub).
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -tags webembed \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/kazper ./cmd/kazper

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/kazper /app/kazper
WORKDIR /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/kazper"]
CMD ["serve"]
