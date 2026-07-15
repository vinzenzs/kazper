# supplements Specification

## Purpose
TBD - created by archiving change add-supplement-log. Update Purpose after archive.
## Requirements
### Requirement: Supplement intakes are logged as timestamped events

The system SHALL persist supplement intakes in a `supplement_entries` table: `logged_at`
(TIMESTAMPTZ NOT NULL), required free-text `name`, optional `dose` (NUMERIC, > 0) with paired
`dose_unit` (TEXT) — both present or both absent — an optional `note`, and audit timestamps.
Multiple entries per day SHALL be allowed. `POST /api/v1/supplements` SHALL create entries with
the standard `Idempotency-Key` supported; a missing name SHALL return `400 name_required`, an
unpaired dose/unit `400 dose_pair_required`, a non-positive dose `400 dose_invalid`. Supplement
data SHALL feed no nutrition, hydration, or energy total.

#### Scenario: A bare-name entry is valid

- **WHEN** `POST /supplements` carries `{"name":"vitamin D","logged_at":"2026-07-14T08:00:00Z"}`
- **THEN** the entry is stored with no dose and echoed back

#### Scenario: A dosed entry requires the pair

- **WHEN** the body carries `dose: 5` with no `dose_unit`
- **THEN** the response is `400` with `dose_pair_required`

### Requirement: Supplements are readable by window and id, and deletable

The system SHALL expose `GET /api/v1/supplements?from=&to=` returning the inclusive range's
entries ascending by `logged_at` (`200` with empty `entries` when none; the shared range
vocabulary with a 92-day cap), `GET /supplements/{id}` (`404 not_found` unknown), and
`DELETE /supplements/{id}` (`404 not_found`). There SHALL be no PATCH — corrections are delete
plus re-log.

#### Scenario: A window returns ascending entries

- **WHEN** three entries exist across the requested range
- **THEN** they return in ascending `logged_at` order

#### Scenario: No update path exists

- **WHEN** a client attempts `PATCH /supplements/{id}`
- **THEN** the route does not exist (405/404 per router behavior), by design

### Requirement: Supplements are writable and readable over MCP

The system SHALL expose `log_supplement` (write tier, wraps the POST with auto-derived
idempotency) and `list_supplements` (read tier, window GET), each one HTTP call forwarding the
body verbatim; `log_supplement`'s description SHALL note that corrections are delete + re-log.

#### Scenario: Agent logs a supplement in one call

- **WHEN** the agent invokes `log_supplement` with a name and timestamp
- **THEN** one POST is issued and the stored entry returns verbatim

