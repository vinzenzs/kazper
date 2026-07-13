# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: Athlete-config history tool mirrors the history endpoint

The system SHALL expose one MCP read tool via the shared `agenttools`
registry: `athlete_config_history_get` → `GET /athlete-config/history`, with
optional `from`/`to` args mapped to the query parameters, forwarding the
response body verbatim and sending no `Idempotency-Key` (read tool). The tool
SHALL be registered alongside the existing `athlete_config_get`. The MCP
announced surface is registry-derived, so the tool SHALL appear in the
announced list and in the regenerated announced-schema golden (regenerated
via `-tags=goldengen`, not a hand-edited expected-tools list).

#### Scenario: athlete_config_history_get calls the history endpoint

- **WHEN** the agent calls `athlete_config_history_get` with `from` and `to`
- **THEN** the wrapper issues exactly one `GET /athlete-config/history?from=...&to=...` with no `Idempotency-Key` header
- **AND** returns the response body verbatim

#### Scenario: Bounds are optional

- **WHEN** the agent calls `athlete_config_history_get` with no args
- **THEN** the wrapper issues `GET /athlete-config/history` with no query parameters

#### Scenario: Announced surface includes the history tool

- **WHEN** the MCP server lists its tools
- **THEN** `athlete_config_history_get` is present
- **AND** the registry-equality integration assertion passes with the regenerated schema golden
