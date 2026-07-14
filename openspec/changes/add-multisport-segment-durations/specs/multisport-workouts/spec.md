## ADDED Requirements

### Requirement: Template reads carry per-segment estimated durations

Multisport template read responses (single and list, over REST and the MCP tools that forward
them) SHALL carry on each **sport segment** a derived, response-only `estimated_duration_sec` —
the sum of that segment's time-bound step durations with repeat blocks expanded, `null` when the
segment contains any non-time-bound step — using the same derivation rule as the existing
template-level value, which SHALL remain unchanged. The field SHALL NOT be writable or stored;
transition durations keep their existing explicit representation.

#### Scenario: Fully time-bounded segments report their estimates

- **WHEN** a swim/bike/run template's segments are each fully time-bounded
- **THEN** each segment carries its own `estimated_duration_sec` and the template-level value
  equals their sum plus no contribution from transitions beyond existing semantics

#### Scenario: An unbounded segment is null without hiding its siblings

- **WHEN** the bike segment contains a distance-bound step while swim and run are time-bounded
- **THEN** the bike segment's `estimated_duration_sec` is null, swim and run report values, and
  the template-level value is null as today

#### Scenario: The field is response-only

- **WHEN** a template write supplies `estimated_duration_sec` on a segment
- **THEN** it is ignored (never stored), and reads continue to derive it
