## ADDED Requirements

### Requirement: The daily context response carries active coach memory

The `GET /context/daily` aggregator response SHALL include a `memory` block listing active
coach-memory items relevant to the requested date: every standing item (`status = active`,
not expired, kind in `fact | preference | constraint | observation`) plus any
`recommendation` whose `date` is the requested date. Each item past its `review_at`
(`review_at <= today`) SHALL carry `needs_review: true`. Items with `status = archived` or
`expires_at` before today SHALL be excluded. The aggregator performs no synthesis over
memory — it returns the stored items verbatim.

#### Scenario: Standing facts ride the daily context

- **WHEN** an active `constraint` exists and the client requests `/context/daily` for any date
- **THEN** the `memory` block includes that constraint

#### Scenario: Items due for review are flagged

- **WHEN** an active item's `review_at` is on or before today
- **THEN** its entry in the `memory` block carries `needs_review: true`

#### Scenario: Expired and archived items are excluded

- **WHEN** an item is archived or its `expires_at` is before today
- **THEN** it does not appear in the `memory` block
