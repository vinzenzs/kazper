# coach-recommendations Specification

## Purpose
TBD - created by archiving change persist-coach-recommendations. Update Purpose after archive.
## Requirements
### Requirement: The system persists agent-authored coach recommendations as a dated log

The system SHALL expose `POST /coach/recommendations` (authenticated) recording one coach-authored recommendation with a `date` (the local date the advice applies to, `YYYY-MM-DD`), a `scope` from the validated set `fueling | training | recovery | race | general`, a required non-empty `recommendation` text, and an optional `reason` text. On success it SHALL persist the row and return it (including its generated `id` and timestamps). The endpoint SHALL honor the idempotency middleware on POST. The system SHALL reject a missing/empty `recommendation` with `recommendation_required`, a `scope` outside the set with `scope_invalid`, and an unparseable `date` with `date_invalid` — each a `400`.

#### Scenario: Recording a recommendation

- **WHEN** the client `POST`s `{"date":"2026-06-17","scope":"fueling","recommendation":"Target 220g carbs","reason":"long ride tomorrow"}`
- **THEN** the response is `201` with the stored row carrying a generated `id`, the supplied fields, and `created_at`/`updated_at`

#### Scenario: Recommendation text is required

- **WHEN** the client `POST`s a body with `recommendation` empty or absent
- **THEN** the response is `400 recommendation_required`

#### Scenario: Scope must be in the validated set

- **WHEN** the client `POST`s `{"date":"2026-06-17","scope":"nonsense","recommendation":"x"}`
- **THEN** the response is `400 scope_invalid`

### Requirement: Recommendations are read back over a date window

The system SHALL expose `GET /coach/recommendations` accepting `from`, `to`, `tz`, and an optional `scope`, returning the recommendations whose `date` falls in the inclusive `[from, to]` local-date window (resolved in `tz`, defaulting to the configured zone), ordered newest-first (`date` descending, then `created_at` descending). When `scope` is supplied the list SHALL be narrowed to that scope. Invalid `from`/`to`/`tz` SHALL return `400`.

#### Scenario: Window returns in-range recommendations newest-first

- **WHEN** recommendations exist on several dates and the client requests a window covering some of them
- **THEN** only the in-window rows are returned, ordered newest date first

#### Scenario: Scope filter narrows the list

- **WHEN** the client requests the window with `scope=recovery`
- **THEN** only `recovery`-scoped rows in the window are returned

#### Scenario: Out-of-window rows are excluded

- **WHEN** a recommendation's `date` is outside the requested window
- **THEN** it does not appear in the response

### Requirement: A single recommendation can be fetched and deleted

The system SHALL expose `GET /coach/recommendations/{id}` returning the stored row, and `DELETE /coach/recommendations/{id}` removing it (corrections are a delete followed by a re-log; there is no PATCH). Both SHALL return `404 recommendation_not_found` when no row matches the id.

#### Scenario: Fetch one recommendation

- **WHEN** the client `GET`s `/coach/recommendations/{id}` for an existing row
- **THEN** the response is `200` with that recommendation

#### Scenario: Delete removes the recommendation

- **WHEN** the client `DELETE`s an existing recommendation
- **THEN** the response is `204`
- **AND** a subsequent `GET /coach/recommendations/{id}` returns `404 recommendation_not_found`

#### Scenario: Operating on a missing id is a 404

- **WHEN** the client `GET`s or `DELETE`s an id that does not exist
- **THEN** the response is `404 recommendation_not_found`

### Requirement: The store records primitives only and performs no synthesis

The system SHALL treat a recommendation as a primitive record: it stores the exact `recommendation`/`reason` text the agent authored and returns it verbatim, and SHALL NOT generate, rank, score, interpret, or otherwise derive recommendations server-side. Recording a recommendation SHALL NOT mutate any nutrition goal, daily goal override, or other enforced target — a recommendation is a note, not an enforced number.

#### Scenario: Recording a recommendation does not change enforced targets

- **WHEN** the client records a recommendation mentioning a carb target for a date
- **AND** then calls `GET /goals` and `GET /goals/overrides/{date}` for that date
- **THEN** neither reflects any change from the recommendation (targets change only via the goals/override endpoints)

### Requirement: The recommendation log is mirrored as MCP tools

The system SHALL mirror the recommendation endpoints 1:1 as MCP tools — `log_coach_recommendation` (write), `list_coach_recommendations` (read), `get_coach_recommendation` (read), and `delete_coach_recommendation` (write). Each tool SHALL issue exactly one corresponding HTTP request and forward the response body verbatim; the write tools SHALL auto-derive an idempotency key when the agent supplies none.

#### Scenario: Logging a recommendation via MCP issues one request

- **WHEN** the agent calls `log_coach_recommendation` with a date, scope, and recommendation
- **THEN** the MCP server issues exactly one `POST /coach/recommendations` and returns the response verbatim

#### Scenario: Listing recommendations via MCP issues one request

- **WHEN** the agent calls `list_coach_recommendations` with a window
- **THEN** the MCP server issues exactly one `GET /coach/recommendations` and returns the response verbatim

