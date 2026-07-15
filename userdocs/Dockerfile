# syntax=docker/dockerfile:1
#
# Multi-stage build for the Kazper user guide (MkDocs Material). The build stage
# renders the static site from mkdocs.yml + userdocs/; the final image is the
# same unprivileged nginx the public-site image uses — a static file server with
# no secrets and no path to Kazper.
#
# Build context is the REPO ROOT (mkdocs.yml lives there). Example:
#   docker build -f userdocs/Dockerfile -t ghcr.io/vinzenzs/kazper-docs:latest .

FROM python:3.12-slim AS build
WORKDIR /src
RUN pip install --no-cache-dir mkdocs-material
COPY mkdocs.yml ./
COPY userdocs ./userdocs
RUN mkdocs build --strict

# nginx-unprivileged runs as a non-root user (uid 101) and listens on 8080, so it
# works under the chart's runAsNonRoot pod security context without extra fuss.
FROM nginxinc/nginx-unprivileged:alpine
COPY --from=build /src/site /usr/share/nginx/html
