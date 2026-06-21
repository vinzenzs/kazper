## ADDED Requirements

### Requirement: A macrocycle is a named, dated, optionally race-anchored season

The system SHALL persist macrocycles via a `macrocycles` table with columns
`(id UUID PK, name TEXT NOT NULL, start_date DATE NOT NULL, end_date DATE NOT NULL,
race_id UUID NULL REFERENCES races(id) ON DELETE SET NULL, methodology TEXT NULL,
notes TEXT NULL, created_at, updated_at)`. Date ranges are inclusive on both ends and a
CHECK SHALL enforce `start_date <= end_date`. A macrocycle is a season-level planning
container; it groups training-phases (which carry the per-period date ranges) and SHALL NOT
duplicate those dates. `race_id`, when set, anchors the season to its goal race (the A-race
the season peaks for); `methodology` is curated Markdown stored verbatim (no server-side
rendering), distinct from the operational `notes`. Macrocycles MAY overlap; the
covering-season query resolves ties by most-recently-updated.

#### Scenario: Table is created with the documented shape

- **WHEN** the migration set is applied to a clean database
- **THEN** the `macrocycles` table exists with the documented columns and the
  `race_id` foreign key to `races(id)` `ON DELETE SET NULL`
- **AND** a CHECK constraint enforces `start_date <= end_date`

#### Scenario: POST /macrocycles creates a season

- **WHEN** the client calls `POST /macrocycles` with body `{"name":"2026 road season","start_date":"2026-01-05","end_date":"2026-09-27","race_id":"<race-uuid>","methodology":"Build to a late-September peak…","notes":"A-race = IM 70.3"}`
- **THEN** the system creates the row and returns `201 Created` with body `{"macrocycle": {<the stored row>, "race_name":"<resolved race name>"}}`
- **AND** the response includes `race_name` (the resolved name of the referenced race) as a convenience sibling, or null when `race_id` is unset

#### Scenario: POST validates the date range

- **WHEN** the client POSTs with `start_date > end_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_range_invalid"}`
- **AND** no row is inserted

#### Scenario: POST validates the name

- **WHEN** the client POSTs with an empty or whitespace-only `name`
- **THEN** the system returns `400 Bad Request` with `{"error":"macrocycle_name_invalid"}`

- **WHEN** `name` exceeds 128 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"macrocycle_name_too_long","max_length":128}`

#### Scenario: POST with a non-existent race_id is rejected

- **WHEN** the client POSTs a `race_id` that matches no existing race
- **THEN** the system returns `400 Bad Request` with `{"error":"race_not_found"}`
- **AND** no row is inserted

#### Scenario: POST with race_id omitted creates an unanchored season

- **WHEN** the client POSTs without `race_id`
- **THEN** the macrocycle is created with `race_id = NULL` and `race_name = null`

#### Scenario: methodology and notes are independent and serialize as null when unset

- **WHEN** a macrocycle has neither `methodology` nor `notes`
- **THEN** both fields serialize as null, not an error
- **AND** when a write supplies a new `methodology` without `notes`, `methodology` is
  replaced and `notes` is left unchanged

### Requirement: The macrocycle read returns the season with its ordered member phases

The system SHALL expose `GET /macrocycles/{id}` returning the season together with its
**member phases** — every `training_phases` row whose `macrocycle_id` equals the requested
macrocycle — ordered by `macrocycle_ordinal NULLS LAST, start_date` ascending. Each member
entry SHALL carry the phase's identity, type, date range, `macrocycle_ordinal`, and the
per-period progression targets (`target_weekly_tss`, `target_weekly_hours`) so a single read
presents the whole yearly load progression. Numeric target fields SHALL be rounded at the
response boundary. `GET /macrocycles` SHALL list every macrocycle ordered by `start_date`
descending (the season envelope only, without nested phases).

#### Scenario: GET by id returns the nested progression

- **WHEN** a macrocycle has three phases linked with ordinals 1, 2, 3
- **AND** the client calls `GET /macrocycles/<id>`
- **THEN** the response is `200 OK` with `{"macrocycle": {…, "race_name": <name or null>, "phases": [<phase1>,<phase2>,<phase3>]}}`
- **AND** the phases are ordered by `macrocycle_ordinal` ascending, ties and nulls falling back to `start_date`
- **AND** each phase entry includes its `target_weekly_tss` and `target_weekly_hours` (rounded, or null when unset)

#### Scenario: GET by id on a season with no member phases returns an empty list

- **WHEN** a macrocycle exists but no phase links to it
- **THEN** the response `phases` array is empty (`[]`), not null

#### Scenario: GET on unknown id returns 404

- **WHEN** no macrocycle with that id exists
- **THEN** the system returns `404 Not Found` with `{"error":"macrocycle_not_found"}`

#### Scenario: GET /macrocycles lists all seasons

- **WHEN** the client calls `GET /macrocycles`
- **THEN** the response is `200 OK` with `{"macrocycles": [<macrocycle>, …]}` ordered by `start_date` descending
- **AND** each entry carries `race_name` (resolved, or null) and `phases` as `null` (the nested member set is populated only by the by-id read)

### Requirement: Macrocycles support partial update and deletion that orphans members

The system SHALL expose `PATCH /macrocycles/{id}` updating any subset of
`name`, `start_date`, `end_date`, `race_id`, `methodology`, `notes`, validating the same
rules as create, and `DELETE /macrocycles/{id}` removing the season. Deleting a macrocycle
SHALL set every member phase's `macrocycle_id` (and the season-membership fields' meaning)
to NULL via the `ON DELETE SET NULL` foreign key — the phases survive and continue to drive
adherence unchanged. The `race_id` field SHALL be tri-state on PATCH: a UUID sets the
anchor, an empty string clears it, and omission leaves it unchanged. PATCH with an empty
body SHALL return `400 patch_empty`.

#### Scenario: PATCH updates a subset of fields

- **WHEN** a macrocycle exists and the client `PATCH`es `{"end_date":"2026-10-04","methodology":"Pushed the peak a week."}`
- **THEN** only `end_date` and `methodology` change (and `updated_at` bumps)
- **AND** every other field is preserved
- **AND** the response is `200 OK` with the updated macrocycle

#### Scenario: PATCH tri-state on race_id

- **WHEN** the client `PATCH`es `{"race_id":""}` on a macrocycle that currently has an anchor
- **THEN** `race_id` becomes NULL and `race_name` becomes null
- **AND** a `PATCH` omitting `race_id` leaves the existing anchor unchanged
- **AND** a `PATCH` with a UUID that matches no race returns `400 race_not_found`

#### Scenario: PATCH with an empty body returns 400

- **WHEN** the client `PATCH`es `{}`
- **THEN** the system returns `400 Bad Request` with `{"error":"patch_empty"}`

#### Scenario: DELETE orphans member phases rather than deleting them

- **WHEN** a macrocycle has two member phases and the client `DELETE`s it
- **THEN** the system returns `204 No Content`
- **AND** both phases still exist with `macrocycle_id = NULL` (and their progression targets and adherence behavior unchanged)
- **AND** a subsequent `GET /macrocycles/<id>` returns `404 macrocycle_not_found`

#### Scenario: DELETE on unknown id returns 404

- **WHEN** no macrocycle with that id exists
- **THEN** the system returns `404 Not Found` with `{"error":"macrocycle_not_found"}`

#### Scenario: Idempotency-Key behavior matches the standard write paths

- **WHEN** the client supplies an `Idempotency-Key` on `POST`/`PATCH`/`DELETE /macrocycles`
- **THEN** the request is handled idempotently by the standard middleware (there is no PUT on this surface, so no `idempotency_unsupported_for_put` case applies)

### Requirement: Macrocycle management is mirrored 1:1 by MCP tools

The system SHALL expose five MCP tools mirroring the macrocycle REST surface —
`create_macrocycle`, `list_macrocycles`, `get_macrocycle`, `update_macrocycle`,
`delete_macrocycle` — each issuing exactly one HTTP call to the corresponding endpoint and
forwarding the response verbatim. The write tools SHALL carry the standard auto-derived
idempotency-key behavior of the shared agent-tool registry. Macrocycles SHALL NOT enter the
goals resolver or plan materialization; the MCP surface is read/author only.

#### Scenario: The MCP surface announces the macrocycle tools

- **WHEN** the MCP server lists its tools
- **THEN** the announced set includes `create_macrocycle`, `list_macrocycles`, `get_macrocycle`, `update_macrocycle`, and `delete_macrocycle`

#### Scenario: get_macrocycle returns the nested progression verbatim

- **WHEN** the agent calls `get_macrocycle` with a macrocycle id
- **THEN** the tool issues `GET /macrocycles/{id}` and returns the season with its ordered member phases exactly as the REST endpoint produced them
