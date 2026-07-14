## ADDED Requirements

### Requirement: A daily subjective wellness entry is stored per date

The system SHALL store at most one wellness entry per calendar date with five OPTIONAL integer
scores constrained to 1–5 — `fatigue`, `soreness`, `stress` (1 = none → 5 = severe) and `mood`,
`motivation` (1 = low → 5 = high) — plus an OPTIONAL free-text `note` (capped at 2000
characters). An entry SHALL be rejected with `400 wellness_empty` when every field is absent.
Out-of-range or non-integer scores SHALL be rejected with `400 wellness_score_invalid` and the
offending `field`; an over-long note with `400 note_too_long`. Score fields SHALL serialize with
`omitempty` — absent means not reported, and no field is ever defaulted.

#### Scenario: A partial entry is first-class

- **WHEN** `PUT /api/v1/wellness/2026-07-14` carries only `{"soreness": 4}`
- **THEN** the entry is stored with `soreness = 4`, all other scores absent, and the response
  echoes only the provided fields

#### Scenario: An empty entry is rejected

- **WHEN** the PUT body carries no score and no note
- **THEN** the response is `400` with `{"error":"wellness_empty"}`

#### Scenario: An out-of-range score names its field

- **WHEN** the body carries `{"mood": 7}`
- **THEN** the response is `400` with `wellness_score_invalid` and `field: "mood"`

### Requirement: Wellness entries are upserted per date with PUT full-replace semantics

The system SHALL expose `PUT /api/v1/wellness/{date}` creating or **fully replacing** that
date's entry (fields absent from the body are cleared, not preserved), `GET /wellness/{date}`
returning the entry or `404 not_found`, and `DELETE /wellness/{date}` removing it
(`404 not_found` when absent). The date path segment SHALL be validated
(`400 date_invalid`). Per the PUT idempotency rule, a request carrying an `Idempotency-Key`
header SHALL be rejected with `400 idempotency_unsupported_for_put`.

#### Scenario: Re-PUT replaces rather than merges

- **WHEN** a date holds `{"fatigue": 3, "note": "heavy legs"}` and a new PUT carries only
  `{"mood": 4}`
- **THEN** the stored entry has `mood = 4` and no fatigue or note

#### Scenario: An Idempotency-Key on PUT is rejected

- **WHEN** `PUT /wellness/{date}` carries an `Idempotency-Key` header
- **THEN** the response is `400` with `idempotency_unsupported_for_put`

#### Scenario: Reading an unlogged date is a 404

- **WHEN** `GET /wellness/{date}` targets a date with no entry
- **THEN** the response is `404` with `not_found`

### Requirement: Wellness entries are readable over a window

The system SHALL expose `GET /api/v1/wellness?from=&to=` returning that inclusive date range's
entries in ascending date order, `200` with an empty `entries` array when none exist (no 404).
Invalid dates SHALL return `400 date_invalid` with the offending `field`; a reversed range
`400 range_invalid`; a range longer than **92 days** `400 range_too_large` with `max_days`.

#### Scenario: A window returns ascending entries

- **WHEN** entries exist on three dates in the requested range
- **THEN** the response carries exactly those three entries in ascending date order

#### Scenario: An empty window is 200, not 404

- **WHEN** no entries exist in the range
- **THEN** the response is `200` with `{"entries":[]}`

### Requirement: Wellness is writable and readable over MCP

The system SHALL expose a `log_wellness` MCP tool wrapping `PUT /api/v1/wellness/{date}` (one
call, full-replace semantics stated in the description, no idempotency key) and a
`list_wellness` read tool wrapping the window GET, each forwarding the response body verbatim.
The `log_wellness` description SHALL state the score directions and encourage partial entries
over skipped days.

#### Scenario: The agent logs a morning check-in in one call

- **WHEN** the agent invokes `log_wellness` with a date and `{"fatigue": 2, "motivation": 5}`
- **THEN** the tool issues one PUT and returns the stored entry verbatim

#### Scenario: The agent reads the recent block

- **WHEN** the agent invokes `list_wellness` with `from`/`to`
- **THEN** the tool issues one window GET and returns the ascending entries verbatim
