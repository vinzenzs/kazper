## ADDED Requirements

### Requirement: A phase carries optional macrocycle membership and progression targets

The system SHALL allow a `training_phases` row to participate in a season (macrocycle) and
to carry per-period training-load targets, via four new nullable columns:
`macrocycle_id UUID NULL REFERENCES macrocycles(id) ON DELETE SET NULL` (the season this
phase belongs to), `macrocycle_ordinal INT NULL` (the phase's position in the season's
progression), `target_weekly_tss NUMERIC NULL`, and `target_weekly_hours NUMERIC NULL` (the
deliberate per-period load targets). All four are optional — an unlinked phase with no
targets is a valid state and the existing adherence behavior is unchanged. The phase create
and update paths SHALL accept these fields, and the phase read paths SHALL return them
(`omitempty` / null when unset), with the numeric targets rounded at the response boundary.
`macrocycle_id` SHALL follow the established tri-state convention on update: a UUID sets the
link, an empty string clears it, and omission leaves it unchanged. A `macrocycle_id` that
matches no existing macrocycle SHALL be rejected with `macrocycle_not_found`; a negative
`target_weekly_tss` or `target_weekly_hours` SHALL be rejected with `target_invalid`. These
fields are declared plan, NOT measured: `target_weekly_tss`/`target_weekly_hours` are never
compared against actual load, and `macrocycle_id` does NOT change which goals drive
adherence — phases continue to resolve adherence solely through `default_template_id`.

#### Scenario: New columns are nullable with no back-fill

- **WHEN** the migration adding the four columns is applied to a database with existing `training_phases` rows
- **THEN** every existing row carries `macrocycle_id = NULL`, `macrocycle_ordinal = NULL`, `target_weekly_tss = NULL`, and `target_weekly_hours = NULL`
- **AND** the migration succeeds without back-filling, and subsequent create/update paths default the fields to NULL when omitted
- **AND** the existing adherence-resolution behavior is unchanged for every date

#### Scenario: POST /phases accepts macrocycle membership and targets

- **WHEN** the client POSTs a phase with `{"name":"build-block-1","type":"build","start_date":"2026-03-02","end_date":"2026-03-29","macrocycle_id":"<season-uuid>","macrocycle_ordinal":2,"target_weekly_tss":620,"target_weekly_hours":11.5}`
- **THEN** the phase is created carrying all four fields
- **AND** the response echoes them (`target_weekly_tss`/`target_weekly_hours` rounded to 1dp)

#### Scenario: POST with a non-existent macrocycle_id is rejected

- **WHEN** the client POSTs a phase with a `macrocycle_id` that matches no macrocycle
- **THEN** the system returns `400 Bad Request` with `{"error":"macrocycle_not_found"}`
- **AND** no phase row is inserted

#### Scenario: POST with a negative target is rejected

- **WHEN** the client POSTs a phase with `target_weekly_tss` or `target_weekly_hours` below zero
- **THEN** the system returns `400 Bad Request` with `{"error":"target_invalid"}`
- **AND** no phase row is inserted

#### Scenario: PATCH tri-state on macrocycle_id

- **WHEN** the client `PATCH`es `{"macrocycle_id":""}` on a phase currently linked to a season
- **THEN** the phase's `macrocycle_id` becomes NULL (it leaves the season)
- **AND** a `PATCH` supplying a UUID re-links it, and a `PATCH` omitting `macrocycle_id` leaves the link unchanged

#### Scenario: PATCH updates progression targets independently

- **WHEN** the client `PATCH`es `{"target_weekly_tss":680}` on an existing phase
- **THEN** only `target_weekly_tss` changes; `target_weekly_hours`, `macrocycle_id`, `macrocycle_ordinal`, and every other field are preserved

#### Scenario: GET returns membership and targets when set

- **WHEN** the client fetches a phase that is linked to a season and has targets
- **THEN** the response includes `macrocycle_id`, `macrocycle_ordinal`, `target_weekly_tss`, and `target_weekly_hours`
- **AND** a phase with none of them set omits/nulls those fields

#### Scenario: Linking a phase to a season does not change its adherence

- **WHEN** a phase that drives adherence via `default_template_id` is linked to a macrocycle
- **AND** the client requests `GET /summary/daily` for a date in the phase
- **THEN** the resolved goals and `goal_source` are identical to before the link (membership is planning metadata only)

#### Scenario: The phase-write MCP tools carry the new fields

- **WHEN** the agent writes a phase via `create_phase` or `update_phase` with `macrocycle_id`, `macrocycle_ordinal`, `target_weekly_tss`, or `target_weekly_hours`
- **THEN** the phase is persisted with those values and the read returns them (tri-state empty-string-clears applies to `macrocycle_id` on `update_phase`)
