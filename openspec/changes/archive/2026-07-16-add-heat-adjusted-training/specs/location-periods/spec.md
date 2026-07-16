## ADDED Requirements

### Requirement: Location writes accept a geocoded place name

`POST /api/v1/locations` (and the `log_location_period` MCP tool) SHALL accept an optional
`place` string as an alternative to explicit coordinates: the system geocodes it via the
weather client's `Geocode` and stores the top match's coordinates and resolved name. No match →
`400 place_not_found`; geocoding unavailable → `503 geocoding_unavailable` (the write is
refused rather than stored ungeocoded). Explicit `lat`/`lon` continue to work and take
precedence when both are supplied.

#### Scenario: A trip is logged by name alone

- **WHEN** `POST /locations` carries `{"place":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28"}`
- **THEN** the stored period carries Mallorca's geocoded coordinates and resolved name

#### Scenario: An unknown place is rejected

- **WHEN** `place` matches nothing
- **THEN** the response is `400` with `place_not_found` and nothing is stored
