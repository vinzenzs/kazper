## ADDED Requirements

### Requirement: Threshold sources are settable over MCP

The MCP server SHALL expose a `set_threshold_sources` write tool issuing a single
`PUT /api/v1/athlete-config/sources` with the full replacement list and forwarding the response
verbatim. The tool description SHALL state the whitelisted field tokens, that the list is
full-replace (empty = all manual), that flipping a source changes the thresholds computations
use without touching confirmed values or threshold history, and SHALL point at
`recompute_workout_tss` when derived TSS should follow the flip.

#### Scenario: The coach sets the policy in one call

- **WHEN** the agent invokes `set_threshold_sources` with `["ftp_watts", "hr_zones"]`
- **THEN** the tool issues one PUT and returns the stored policy verbatim
