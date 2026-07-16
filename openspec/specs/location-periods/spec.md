# location-periods Specification

## Purpose

Answer "where is the athlete on date X" — the input every weather and heat read needs before it
can ask a forecast anything. Home is the default and lives in configuration (`HOME_LAT`/
`HOME_LON`) because it is quasi-static infrastructure; this capability carries the travel layer
as dated, city-grade location periods the coach logs conversationally ("Mallorca, July 20–28").

Resolution is the product, not the log. `LocationOn(date)` is total and deterministic: the
covering period with the latest `start_date`, else configured home, else honestly unconfigured.
Overlaps are accepted rather than rejected — a weekend trip nested inside a training camp is a
real thing to log — and the latest-start rule makes the nesting unambiguous instead of an
error. `GET /locations/resolve` exposes that exact primitive, so a surprising forecast is one
call from an explanation and the endpoint can never disagree with what a consumer actually used.

The capability is deliberately coarse. It stores no GPS and infers nothing from activity data
(a standing non-goal): heat planning needs a city, not a track. It has no PATCH — these are
small, dated, coach-written rows, so corrections and trip extensions are delete plus re-log.
And because nothing downstream is precomputed, a session scheduled months ago follows a trip
the moment that trip is logged.

## Requirements
### Requirement: Travel periods are logged as dated location ranges

The system SHALL persist location periods in a `location_periods` table:
`start_date`/`end_date` (inclusive DATEs, `end >= start` → else `400 range_invalid`), required
`name`, required `lat` ∈ [-90, 90] and `lon` ∈ [-180, 180] (`400 lat_lon_invalid`), optional
`note`, audit timestamps. `POST /api/v1/locations` SHALL create periods (standard
`Idempotency-Key`); `GET /locations?from=&to=` SHALL return periods overlapping the window
ascending by `start_date` (`200` with empty `entries`; shared range vocabulary, 400-day cap);
`GET /locations/{id}` and `DELETE /locations/{id}` SHALL behave per convention
(`404 not_found`). There SHALL be no PATCH — corrections are delete plus re-log. Overlapping
periods SHALL be accepted. The table SHALL be classified export-included.

#### Scenario: A training camp is logged conversationally

- **WHEN** `POST /locations` carries `{"name":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28","lat":39.57,"lon":2.65}`
- **THEN** the period is stored and echoed

#### Scenario: Coordinates are validated

- **WHEN** the body carries `lat: 95`
- **THEN** the response is `400` with `lat_lon_invalid`

### Requirement: A date resolves to exactly one effective location

The system SHALL resolve `LocationOn(date)` as: the covering location period with the latest
`start_date` when one exists, else the configured home location (`HOME_LAT`/`HOME_LON`,
validated as both-or-neither at startup), else unconfigured. `GET /api/v1/locations/resolve?date=`
SHALL expose the resolution as `{lat, lon, name, source: "travel"|"home"}` with `name: "home"`
for the home fallback, and SHALL return `404 location_unconfigured` when neither exists.
Weather consumers SHALL use the same resolution primitive so the endpoint and behavior can
never disagree.

#### Scenario: A trip covers its dates

- **WHEN** a Mallorca period covers 2026-07-22 and home is configured
- **THEN** `resolve?date=2026-07-22` returns Mallorca with `source: "travel"`, and
  `resolve?date=2026-07-30` returns `source: "home"`

#### Scenario: Nested trips resolve to the latest start

- **WHEN** a weekend period (started 2026-07-24) sits inside the camp period (started 2026-07-20)
- **THEN** dates in the weekend resolve to the weekend's location

#### Scenario: Nothing configured is an honest 404

- **WHEN** no period covers the date and home is unconfigured
- **THEN** the response is `404` with `location_unconfigured`

### Requirement: Location periods are writable and readable over MCP

The system SHALL expose `log_location_period` (write tier, wraps the POST; description: the
coach supplies city-grade coordinates and states periods follow the athlete's stated travel)
and `list_location_periods` (read tier, window GET), each one HTTP call forwarding the body
verbatim.

#### Scenario: The agent logs a trip in one call

- **WHEN** the agent invokes `log_location_period` for "Mallorca, July 20–28"
- **THEN** one POST is issued and the stored period returns verbatim

