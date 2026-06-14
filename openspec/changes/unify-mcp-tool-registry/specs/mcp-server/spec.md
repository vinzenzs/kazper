## ADDED Requirements

### Requirement: The MCP tool surface is generated from the shared agent-tool registry

The MCP server SHALL register its entire tool surface by iterating the shared `internal/agenttools` registry rather than from hand-written per-tool registrations, so the desktop coach and the in-app chat coach are the same tools by construction. Each tool's name, description, input schema, idempotency semantics, and REST mapping SHALL come from its registry entry; a single generic handler SHALL execute every tool by building its one `HTTPCall` from the registry entry and dispatching it through the existing API client and error mapping. The registered surface SHALL remain behaviorally identical to the prior hand-written surface — the same tool names and input schemas the agent already relies on. Write tools SHALL derive their idempotency key via the shared `agenttools` derivation (an explicit agent-supplied key still wins). The server's announced tool list SHALL be derived from the registry, and the name-level chat/MCP drift-guard SHALL be retired as redundant once both surfaces originate from the one registry.

#### Scenario: Tools are registered from the registry

- **WHEN** the MCP server starts
- **THEN** it registers exactly the tools in the shared `agenttools` registry (the full surface; the chat-visibility marker and tier are ignored by the MCP server)
- **AND** no tool is registered from a hand-written per-tool handler

#### Scenario: The announced surface is unchanged

- **WHEN** a client calls `tools/list`
- **THEN** the announced tool names and input schemas are identical to those announced before this change
- **AND** the integration test asserts the announced surface equals the registry-derived surface

#### Scenario: A write tool's idempotency key uses the shared derivation

- **WHEN** a write tool is invoked without an explicit `idempotency_key`
- **THEN** the key is computed by the shared `agenttools` derivation (tool name + canonical input)
- **AND** an explicit `idempotency_key` supplied by the agent overrides the derived one

#### Scenario: A tool executes through the generic handler

- **WHEN** any registry tool is invoked
- **THEN** the generic handler builds the tool's single `HTTPCall` from its registry entry, dispatches it via the API client, and maps the response through the existing tool-result/error mapping
- **AND** the result is identical to what the prior bespoke handler produced
