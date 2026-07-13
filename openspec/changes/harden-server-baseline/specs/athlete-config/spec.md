## ADDED Requirements

### Requirement: athlete_config_update MCP tool mirrors PUT /athlete-config

The agent-tool registry SHALL expose an `athlete_config_update` tool that issues `PUT /athlete-config` with a payload mirroring the full REST body. Following the `set_goals` PUT precedent, the tool SHALL NOT attach an idempotency key (the endpoint rejects `Idempotency-Key` on PUT), and its description SHALL state the full-replace semantics (an omitted field is cleared) and the resulting retry-unsafety. The tool SHALL be tiered `write-confirm`, so the chat surface pauses for human confirmation before rewriting physiology config that downstream workout-target resolution consumes. The MCP announced-schema golden SHALL be regenerated additively.

#### Scenario: Agent updates the athlete config over MCP

- **WHEN** the agent calls `athlete_config_update` with a full config payload
- **THEN** the tool issues `PUT /athlete-config` with that body and no `Idempotency-Key` header
- **AND** the REST response body is returned verbatim as the tool result

#### Scenario: Chat surface pauses for confirmation

- **WHEN** the chat loop's assistant turn calls `athlete_config_update`
- **THEN** the turn pauses as a write-confirm proposal and the call is dispatched only after the user approves

#### Scenario: Announced schema stays golden

- **WHEN** the MCP announced-schema golden test runs after the tool is registered
- **THEN** `athlete_config_update` appears in `announced_schemas.json` and the golden comparison passes
