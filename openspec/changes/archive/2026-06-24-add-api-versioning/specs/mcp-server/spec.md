## MODIFIED Requirements

### Requirement: Configuration is read from environment variables

The system SHALL read its configuration from environment variables at process start and refuse to start when required variables are missing. The REST base URL SHALL include the `/api/v1` version prefix; tool calls join their relative paths against that base.

#### Scenario: Missing AGENT_API_TOKEN halts startup

- **WHEN** the process starts with `AGENT_API_TOKEN` unset or empty
- **THEN** the binary writes an error to stderr identifying the variable
- **AND** exits with a non-zero status code

#### Scenario: NUTRITION_API_URL defaults to the versioned localhost base

- **WHEN** the process starts without `NUTRITION_API_URL` set
- **THEN** tool calls target `http://localhost:8080/api/v1`

#### Scenario: Tool calls resolve under the version prefix

- **WHEN** a tool issues its single REST call (e.g. the daily summary tool)
- **THEN** the request resolves to `<NUTRITION_API_URL>/summary/daily` under the `/api/v1` base (e.g. `http://localhost:8080/api/v1/summary/daily`)

#### Scenario: Per-request timeout is configurable

- **WHEN** the process starts with `MCP_REQUEST_TIMEOUT_SECONDS=20`
- **THEN** the wrapper applies a 20-second timeout on each REST call

#### Scenario: Default per-request timeout is 10 seconds

- **WHEN** the process starts without `MCP_REQUEST_TIMEOUT_SECONDS` set
- **THEN** the wrapper applies a 10-second timeout on each REST call
