## ADDED Requirements

### Requirement: Every API request runs under a server-imposed deadline

The system SHALL enforce a per-request timeout on `/api/v1` routes via a context-deadline middleware, configured by `HTTP_REQUEST_TIMEOUT` (default 30s). Routes that legitimately outlive it SHALL be exempted by explicit path prefix: the chat SSE endpoints (which own `CHAT_REQUEST_TIMEOUT`), the meal-photo endpoint (vision upstream), and the Garmin proxy group (which owns per-operation budgets). When the deadline expires before any response bytes are written, the request SHALL fail `504` with error code `request_timeout` in the standard error shape. The underlying `http.Server` SHALL also set `ReadTimeout` and `IdleTimeout` (`WriteTimeout` stays unset to keep SSE streams open).

#### Scenario: A hung handler is cut off at the deadline

- **WHEN** a non-exempt `/api/v1` request's handler blocks beyond `HTTP_REQUEST_TIMEOUT`
- **THEN** the client receives `504` with `{"error": "request_timeout"}`
- **AND** the handler's request context is cancelled so in-flight database work is abandoned

#### Scenario: Chat streaming is exempt

- **WHEN** a `/chat` SSE stream runs longer than `HTTP_REQUEST_TIMEOUT`
- **THEN** the stream is not interrupted by the per-request middleware and remains governed by the chat request timeout

### Requirement: Request bodies are size-capped

The system SHALL cap request bodies on `/api/v1` routes at `MAX_REQUEST_BODY_BYTES` (default 1 MiB) using `http.MaxBytesReader`, returning `413` with error code `body_too_large` when exceeded. Routes with their own caps (meal photo via `MEAL_FROM_PHOTO_MAX_BYTES`, Garmin library import) SHALL be exempt from the global cap so their existing limits govern.

#### Scenario: Oversized JSON body is rejected

- **WHEN** a client sends a JSON body larger than `MAX_REQUEST_BODY_BYTES` to a non-exempt write endpoint
- **THEN** the response is `413` with `{"error": "body_too_large"}` and the handler does not process the payload

#### Scenario: Photo uploads keep their own larger cap

- **WHEN** a meal photo between the global cap and `MEAL_FROM_PHOTO_MAX_BYTES` is uploaded
- **THEN** the upload is accepted (the photo route's own cap governs)

### Requirement: Requests carry a correlation id and 5xx logs are errors

The system SHALL assign every request an id — honoring an inbound `X-Request-ID` header, otherwise generating one — echo it in the `X-Request-ID` response header, and include it in the structured request-completion log line. Completion lines for responses with status ≥ 500 SHALL log at Error level (Info otherwise). The chat loopback dispatcher SHALL propagate the parent request's id onto its in-process tool subrequests so one chat turn's tool fan-out shares one id.

#### Scenario: Generated id is echoed and logged

- **WHEN** a request arrives without an `X-Request-ID` header
- **THEN** the response carries a generated `X-Request-ID`
- **AND** the request log line includes that id

#### Scenario: 5xx elevates the log level

- **WHEN** a request completes with status 500
- **THEN** its completion line is logged at Error level with the request id

#### Scenario: Chat tool calls share the turn's id

- **WHEN** a `/chat` turn dispatches tool calls through the loopback dispatcher
- **THEN** each subrequest's log line carries the same request id as the parent `/chat` request

### Requirement: Opt-in Prometheus metrics endpoint

The system SHALL expose Prometheus metrics at `GET /metrics` (root-level infra route, outside bearer auth, sibling of `/healthz`) **only when** `METRICS_ENABLED=true`; the default is disabled and the route is not registered. When enabled, a middleware SHALL record request count and duration histograms labeled by route template (never raw path), method, and status class, alongside the standard Go runtime collectors.

#### Scenario: Disabled by default

- **WHEN** the server runs without `METRICS_ENABLED` set
- **THEN** `GET /metrics` returns `404`

#### Scenario: Enabled endpoint serves request histograms by route template

- **WHEN** the server runs with `METRICS_ENABLED=true` and has served `/api/v1/meals/{id}` requests
- **THEN** `GET /metrics` returns Prometheus text exposition containing request-duration series labeled with the route template `/api/v1/meals/:id`, not the raw id-bearing path
