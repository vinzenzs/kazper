# coach-memory Specification

## Purpose
A durable, athlete-scoped store the coach writes facts / preferences / constraints / observations / recommendations into and both surfaces (in-app chat coach + MCP agent) read at grounding time — the cross-surface channel that lets one surface know what the other was told, without sharing conversation transcripts. A storage primitive only: it records authored text verbatim, performs no synthesis, and never mutates an enforced target. A recommendation is the dated `kind`; the others are dateless standing items with a review/expire lifecycle. Supersedes the former `coach-recommendations` capability.

## Requirements
### Requirement: The system persists agent-authored coach memory as a kinded, athlete-scoped store

The system SHALL expose `POST /coach/memory` (authenticated) recording one coach-authored
memory item with a `kind` from the validated set `fact | preference | constraint |
observation | recommendation`, a required non-empty `text`, an optional `reason`, an
optional `scope` (the same `fueling | training | recovery | race | general` set, now
optional metadata), an optional `date` (`YYYY-MM-DD`), and optional lifecycle fields
`expires_at` and `review_at` (`YYYY-MM-DD`). When `kind = recommendation` a `date` SHALL be
required; for the other kinds `date` is optional. On success it SHALL persist the row with
`status = active` and return it (including generated `id` and timestamps). The endpoint
SHALL honor the idempotency middleware on POST. The system SHALL reject missing/empty `text`
with `text_required`, a `kind` outside the set with `kind_invalid`, a `scope` outside the
set with `scope_invalid`, an unparseable date field with `date_invalid`, and a
`recommendation` lacking a `date` with `date_required` — each a `400`. The store records the
authored text verbatim and SHALL NOT generate, rank, score, or interpret memory
server-side, and recording memory SHALL NOT mutate any nutrition goal, daily goal override,
or other enforced target.

#### Scenario: Recording a standing fact

- **WHEN** the client `POST`s `{"kind":"constraint","text":"Right knee niggle, easy running only","review_at":"2026-07-05"}`
- **THEN** the response is `201` with the stored row carrying a generated `id`, `status:"active"`, the supplied fields, and `created_at`/`updated_at`
- **AND** no `date` is required because the kind is not `recommendation`

#### Scenario: Recording a recommendation still requires a date

- **WHEN** the client `POST`s `{"kind":"recommendation","scope":"fueling","text":"Target 220g carbs"}` with no `date`
- **THEN** the response is `400 date_required`

#### Scenario: text is required

- **WHEN** the client `POST`s a body with `text` empty or absent
- **THEN** the response is `400 text_required`

#### Scenario: kind must be in the validated set

- **WHEN** the client `POST`s `{"kind":"reminder","text":"x"}`
- **THEN** the response is `400 kind_invalid`

### Requirement: Memory is read back over a window with kind and scope filters

The system SHALL expose `GET /coach/memory` accepting `from`, `to`, `tz`, an optional
`scope`, an optional `kind`, and an optional `include_archived` (default false), returning
items ordered newest-first. Dated `recommendation` items SHALL be filtered to the inclusive
`[from, to]` local-date window; dateless standing items SHALL be returned regardless of the
window (they are not anchored to a date). When `scope` or `kind` is supplied the list SHALL
be narrowed accordingly. By default `status = archived` rows and items whose `expires_at` is
before today SHALL be excluded; `include_archived=true` SHALL include archived rows. Invalid
`from`/`to`/`tz` SHALL return `400`.

#### Scenario: Standing items are returned regardless of window

- **WHEN** a `constraint` with no `date` exists and the client requests any window
- **THEN** the constraint appears in the response

#### Scenario: Recommendations are window-filtered

- **WHEN** a `recommendation` dated outside the requested window exists
- **THEN** it does not appear in the response

#### Scenario: Expired items are excluded by default

- **WHEN** an item's `expires_at` is before today
- **THEN** it is absent from the default list

### Requirement: A memory item can be fetched, confirmed/updated in place, and deleted

The system SHALL expose `GET /coach/memory/{id}` returning the stored row, `PATCH
/coach/memory/{id}` updating only the lifecycle fields `review_at`, `expires_at`, and
`status` in place (leaving `created_at` intact), and `DELETE /coach/memory/{id}` removing
it. The PATCH SHALL NOT edit `text`, `kind`, `scope`, or `date` — a content correction is a
delete followed by a re-log. All three SHALL return `404 memory_not_found` when no row
matches the id. PATCH SHALL reject a `status` outside `active | archived` with
`status_invalid` and an unparseable date field with `date_invalid`.

#### Scenario: Confirming a fact pushes its review date without resetting created_at

- **WHEN** the client `PATCH`es `{"review_at":"2026-07-20"}` on an existing item
- **THEN** the response is `200` with the new `review_at`
- **AND** `created_at` is unchanged (the original first-seen time is preserved)

#### Scenario: PATCH rejects content edits

- **WHEN** the client `PATCH`es `{"text":"different"}`
- **THEN** the request is rejected (content is immutable via PATCH; correct via delete + re-log)

#### Scenario: Archiving hides an item from the default list

- **WHEN** the client `PATCH`es `{"status":"archived"}` on an item
- **THEN** a subsequent default `GET /coach/memory` omits it
- **AND** `GET /coach/memory?include_archived=true` includes it

#### Scenario: Operating on a missing id is a 404

- **WHEN** the client `GET`s, `PATCH`es, or `DELETE`s an id that does not exist
- **THEN** the response is `404 memory_not_found`

### Requirement: Coach memory is mirrored 1:1 as MCP tools, write tools are explicit

The system SHALL mirror the memory endpoints as MCP tools — a write tool for `POST`, a
list tool for the windowed read, a get tool, an update tool for the `PATCH`, and a delete
tool — each issuing exactly one corresponding HTTP request and forwarding the response body
verbatim, with write tools auto-deriving an idempotency key when none is supplied. Memory
writes SHALL be explicit: never written from inferred conversation, only on a user-initiated
"remember…" (MCP) or a user-confirmed proposal (chat write-confirm tier).

#### Scenario: Remembering via MCP issues one request

- **WHEN** the agent calls the memory write tool with a kind and text
- **THEN** the MCP server issues exactly one `POST /coach/memory` and returns the response verbatim

#### Scenario: Confirming via MCP issues one request

- **WHEN** the agent calls the memory update tool with a new `review_at`
- **THEN** the MCP server issues exactly one `PATCH /coach/memory/{id}` and returns the response verbatim

