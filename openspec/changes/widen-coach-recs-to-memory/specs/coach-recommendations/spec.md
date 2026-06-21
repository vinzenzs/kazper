## REMOVED Requirements

### Requirement: The system persists agent-authored coach recommendations as a dated log

**Reason**: Widened into the `coach-memory` capability — a recommendation is now one
`kind` of memory alongside facts, preferences, constraints, and observations.
**Migration**: `coach_recommendations` rows are migrated to `coach_memory` with
`kind = 'recommendation'`, `text` = the old `recommendation`, and `status = 'active'`;
`date`/`scope`/`reason` carry over unchanged.

### Requirement: Recommendations are read back over a date window

**Reason**: Superseded by `GET /coach/memory`, which window-filters `recommendation` items
and returns standing items regardless of window.
**Migration**: callers move from `GET /coach/recommendations` to `GET /coach/memory`
(optionally `kind=recommendation` to preserve the old scope).

### Requirement: A single recommendation can be fetched and deleted

**Reason**: Superseded by `GET/PATCH/DELETE /coach/memory/{id}` (the memory item adds an
in-place lifecycle PATCH the recommendation log deliberately lacked).
**Migration**: `GET`/`DELETE /coach/recommendations/{id}` → `/coach/memory/{id}`.

### Requirement: The store records primitives only and performs no synthesis

**Reason**: Carried forward verbatim into the `coach-memory` capability (the store still
records primitives and never synthesizes or mutates enforced targets).
**Migration**: none — the guarantee is preserved under `coach-memory`.

### Requirement: The recommendation log is mirrored as MCP tools

**Reason**: The `*_coach_recommendation(s)` tools are renamed into the `coach-memory` tool
family (a breaking rename; single-user with both clients owned).
**Migration**: re-point the MCP client to the memory tools; bump the `mcp_integration_test`
expected-tools list.
