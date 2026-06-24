## MODIFIED Requirements

### Requirement: OpenAPI specification generation

The system SHALL generate an OpenAPI 2.0 specification from annotations on HTTP handler functions, covering every public endpoint exposed under `/products`, `/meals`, and `/summary`. All documented domain endpoints SHALL be served under the `/api/v1` base path; the generated specification's base path SHALL be `/api/v1`. Infra endpoints (`/healthz`, `/readyz`) and the interactive docs UI (`/swagger`) are NOT versioned and remain at the root path.

The generated specification SHALL include, for each endpoint: HTTP method, path, summary, tags, request parameters (path, query, body), success response schema, and documented error responses (at minimum 400, 401, 404 where applicable).

The generated artifacts SHALL be committed to the repository under `docs/` so that building the binary does not require the documentation toolchain.

#### Scenario: Annotated endpoint appears in the spec

- **WHEN** a handler function in `internal/products`, `internal/meals`, or `internal/summary` is registered on the Gin router with `swag` annotations
- **THEN** running `task swag` regenerates `docs/swagger.json`, `docs/swagger.yaml`, and `docs/docs.go` containing an entry for that endpoint with its method, path, summary, parameters, and response schemas
- **AND** the generated base path is `/api/v1`

#### Scenario: Infra endpoints are unversioned

- **WHEN** the server registers `/healthz`, `/readyz`, and `/swagger`
- **THEN** they are served at the root path, not under `/api/v1`

#### Scenario: Regenerated docs match committed docs

- **WHEN** a developer runs `task swag` against an unchanged source tree
- **THEN** the resulting `docs/` files are byte-identical to the committed versions (so a CI diff check would pass)
