# race-fueling-plan delta — add-race-priority

## MODIFIED Requirements

### Requirement: Persistent race with ordered legs

The system SHALL persist a race as `{name, race_date, race_type?, location?,
notes?, priority?}` owning an ordered list of legs, each
`{ordinal, discipline, distance_m?, expected_duration_min?, intensity?}`.
`discipline` MUST be one of `swim`, `bike`, `run`, `transition`, `other`.
Legs MUST be uniquely ordered within a race (`ordinal` unique per race) and MUST
be deleted when their race is deleted. The system SHALL expose create, read,
list, update, and delete over races.

#### Scenario: Create a race with legs

- **WHEN** the client POSTs a race with `name`, `race_date`, and a `legs` array
  of `{ordinal, discipline, expected_duration_min}`
- **THEN** the race and its legs are persisted and returned with a generated
  `id` and the legs ordered by `ordinal` ascending

#### Scenario: Deleting a race removes its legs

- **WHEN** a race with legs is deleted
- **THEN** the race and all its `race_legs` rows are removed
- **AND** a subsequent `GET /races/{id}` returns `404 race_not_found`

#### Scenario: Duplicate leg ordinal is rejected

- **WHEN** a race is created or updated with two legs sharing the same `ordinal`
- **THEN** the request is rejected with `400 leg_ordinal_duplicate`
- **AND** nothing is persisted

#### Scenario: Invalid discipline is rejected

- **WHEN** a leg is supplied with a `discipline` outside the allowed set
- **THEN** the request is rejected with `400 leg_discipline_invalid`

## ADDED Requirements

### Requirement: Race priority is advisory A/B/C triage metadata

The system SHALL support an optional `priority` field on the race, whose value
MUST be one of `A`, `B`, or `C` (TrainingPeaks-style triage: A = full taper +
peak, B = mini-taper, C = train through). The field SHALL be nullable with no
default — an absent priority means the race is not yet triaged — and SHALL be
persisted with a database CHECK constraint enforcing the closed set. `priority`
SHALL be writable on create and on PATCH; on PATCH it SHALL be tri-state: a
valid value sets it, an empty string clears it to null, and omission leaves it
unchanged. A value outside the closed set SHALL be rejected with
`400 race_priority_invalid` using the structured error shape. When set, the
field SHALL be returned everywhere the race row is serialized (create and PATCH
echoes, `GET /races/{id}`, `GET /races`); when unset it SHALL be omitted from
the JSON. Priority is advisory metadata for the coach agent: the system SHALL
NOT enforce consistency with the macrocycle's A-race anchor (`macrocycles.race_id`)
in either write direction.

#### Scenario: Create a race with a priority

- **WHEN** the client POSTs a race with `name`, `race_date`, and `"priority":"A"`
- **THEN** the race is persisted with priority `A`
- **AND** the create response and subsequent `GET /races/{id}` both include
  `"priority":"A"`

#### Scenario: Invalid priority is rejected

- **WHEN** a race is created or PATCHed with a `priority` outside `A|B|C`
  (e.g. `"D"`, `"a"`, or `"high"`)
- **THEN** the request is rejected with `400` and body `{"error":"race_priority_invalid"}`
- **AND** nothing is persisted or changed

#### Scenario: PATCH priority is tri-state

- **WHEN** the client PATCHes `{"priority":"B"}` on an existing race
- **THEN** the race's priority becomes `B` and every other field is preserved

- **WHEN** the client PATCHes `{"priority":""}` on a race that has a priority
- **THEN** the priority is cleared to null and the response omits `priority`

- **WHEN** the client PATCHes a race without a `priority` key
- **THEN** the existing priority is left unchanged

#### Scenario: Untriaged races omit the field

- **WHEN** a race that has never been given a priority is read via
  `GET /races/{id}` or `GET /races`
- **THEN** its JSON contains no `priority` key

#### Scenario: Priority does not couple to the macrocycle anchor

- **WHEN** a macrocycle anchors race X via `race_id` and the client PATCHes
  race X to `"priority":"C"`
- **THEN** the PATCH succeeds with `200 OK`
- **AND** the macrocycle's anchor is unchanged and no error or warning is raised

### Requirement: Race list filters by priority

`GET /races` SHALL accept an optional `priority` query parameter. When supplied
with a valid value (`A`, `B`, or `C`), the response SHALL contain only races
whose stored priority equals that value. When the parameter is omitted, the
list SHALL return all races unchanged (including untriaged ones). A supplied
value outside the closed set SHALL be rejected with `400 race_priority_invalid`.

#### Scenario: Filter returns only matching races

- **WHEN** races exist with priorities `A`, `C`, and null, and the client calls
  `GET /races?priority=A`
- **THEN** the response contains only the `A`-priority race(s)

#### Scenario: Omitted filter returns everything

- **WHEN** the client calls `GET /races` with no `priority` parameter
- **THEN** all races are returned, including those with no priority

#### Scenario: Invalid filter value is rejected

- **WHEN** the client calls `GET /races?priority=X`
- **THEN** the response is `400` with body `{"error":"race_priority_invalid"}`
