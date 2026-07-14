# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: Race pacing tools mirror the pacing REST endpoints

The system SHALL expose three MCP tools via the shared `agenttools` registry,
each invoking exactly one pacing REST endpoint: `plan_race_pacing` →
`GET /races/{id}/pacing-plan`, `set_race_leg_pacing_override` →
`PUT /races/{id}/pacing-plan/overrides/{ordinal}`, and
`clear_race_leg_pacing_override` →
`DELETE /races/{id}/pacing-plan/overrides/{ordinal}`, forwarding the response
verbatim. `plan_race_pacing` is a read tool and SHALL NOT send an
`Idempotency-Key`; `set_race_leg_pacing_override` maps to a PUT, on which the
dispatcher SHALL skip the idempotency header (consistent with
`set_daily_goal_override`); `clear_race_leg_pacing_override` follows the
standard write-tool idempotency-key derivation. The MCP announced surface is
registry-derived, so the three tools SHALL appear in the announced list and
the regenerated schema golden.

#### Scenario: plan_race_pacing calls the pacing endpoint

- **WHEN** the agent calls `plan_race_pacing` with a race id
- **THEN** the wrapper issues `GET /races/{id}/pacing-plan` with no
  `Idempotency-Key` header and returns the response body verbatim

#### Scenario: set_race_leg_pacing_override issues a PUT without an idempotency key

- **WHEN** the agent calls `set_race_leg_pacing_override` with a race id,
  ordinal, and a power band
- **THEN** the wrapper issues
  `PUT /races/{id}/pacing-plan/overrides/{ordinal}` with the band as the JSON
  body
- **AND** sends no `Idempotency-Key` header (PUT rule)

#### Scenario: clear_race_leg_pacing_override issues the DELETE

- **WHEN** the agent calls `clear_race_leg_pacing_override` with a race id and
  ordinal
- **THEN** the wrapper issues
  `DELETE /races/{id}/pacing-plan/overrides/{ordinal}` with an
  `Idempotency-Key` header (explicit if supplied, else derived)

#### Scenario: Announced surface includes the pacing tools

- **WHEN** the MCP server lists its tools
- **THEN** `plan_race_pacing`, `set_race_leg_pacing_override`, and
  `clear_race_leg_pacing_override` are present
- **AND** the registry-equality integration assertion passes with the
  regenerated schema golden
