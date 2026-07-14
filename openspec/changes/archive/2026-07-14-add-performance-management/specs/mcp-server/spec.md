# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: pmc_series tool wraps the performance-management endpoint

The system SHALL expose a `pmc_series` read tool, registered via the shared
`agenttools` registry, that issues a single `GET /api/v1/performance/pmc` and
forwards the response body verbatim through the existing tool-result/error
mapping. The tool SHALL accept `from` and `to` (inclusive `YYYY-MM-DD` dates)
and an optional `tz` (IANA timezone), mirroring the REST query 1:1. It is
read-only and SHALL NOT send an `Idempotency-Key`. The tool description SHALL
name the Coggan CTL/ATL/TSB semantics and distinguish the series from the
Garmin-mirrored acute/chronic load in `fitness-metrics`, so the agent picks the
right tool for form/taper questions.

#### Scenario: Agent reads the PMC in one call

- **WHEN** the agent invokes `pmc_series` with a `from`, `to`, and optional
  `tz`
- **THEN** the tool issues exactly one `GET /performance/pmc` request and
  returns the daily CTL/ATL/TSB series, ramp alerts, and missing-TSS counters
  as the tool result

#### Scenario: The tool is announced from the registry

- **WHEN** a client calls `tools/list`
- **THEN** `pmc_series` appears in the announced surface, derived from its
  `agenttools` registry entry (no hand-written registration)
- **AND** its announced input schema matches the regenerated golden baseline

#### Scenario: REST validation errors surface as agent-shaped tool errors

- **WHEN** the agent invokes `pmc_series` with an inverted or oversized window
- **THEN** the tool result carries the REST error body (e.g. `range_invalid`,
  `range_too_large`) with `isError=true`, per the existing error mapping
