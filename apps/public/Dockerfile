# syntax=docker/dockerfile:1
#
# Multi-stage build for the public "road to race" site. The build stage runs the
# Astro build (feed fetch → static gen → OG card) with FEED_SECRET supplied as a
# BuildKit secret, so the secret stays out of every image layer and the build
# history. The final image is nginx serving the baked static files: no secret,
# no FEED_URL, no node toolchain — a static file server with no path to Kazper.
#
# Build context is apps/public. Example (from the repo root):
#   docker build -f apps/public/Dockerfile \
#     --secret id=feed_secret,env=FEED_SECRET \
#     --build-arg FEED_URL="https://kazper.example.com/public/race-feed" \
#     --build-arg SITE_URL="https://race.example.com" \
#     -t ghcr.io/vinzenzs/kazper-public-site:latest apps/public

FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . ./
ARG FEED_URL
ARG SITE_URL
ARG BASE_PATH=/
RUN --mount=type=secret,id=feed_secret \
    FEED_SECRET="$(cat /run/secrets/feed_secret)" \
    FEED_URL="$FEED_URL" SITE_URL="$SITE_URL" BASE_PATH="$BASE_PATH" \
    npm run build

# nginx-unprivileged runs as a non-root user (uid 101) and listens on 8080, so it
# works under the chart's runAsNonRoot pod security context without extra fuss.
FROM nginxinc/nginx-unprivileged:alpine
# Static assets only — the feed secret and the Kazper origin never reach here.
COPY --from=build /app/dist /usr/share/nginx/html
