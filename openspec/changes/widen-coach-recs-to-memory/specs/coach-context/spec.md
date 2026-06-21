## ADDED Requirements

### Requirement: The training context response carries active coach memory

The `GET /context/training` aggregator response SHALL include a `memory` block listing
active coach-memory items: every standing item (`status = active`, not expired, kind in
`fact | preference | constraint | observation`) plus any `recommendation` whose `date`
falls in the aggregator's lookback window. Each item past its `review_at`
(`review_at <= today`) SHALL carry `needs_review: true`. Archived and expired items SHALL be
excluded. The aggregator performs no synthesis over memory — items are returned verbatim.
This is what lets the external MCP agent ground on what the in-app coach was told (and
vice-versa) without sharing conversation transcripts.

#### Scenario: Memory grounds the training context

- **WHEN** an active `preference` ("prefers gels over drink mix") exists and the client requests `/context/training`
- **THEN** the `memory` block includes that preference

#### Scenario: Recommendations are window-scoped in training context

- **WHEN** a `recommendation` is dated outside the lookback window
- **THEN** it is absent from the `memory` block, while dateless standing items remain present
