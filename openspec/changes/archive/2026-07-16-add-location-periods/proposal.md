## Why

The weather arc needs to know *where the athlete is on a given date* — home by default, elsewhere during travel — and the user's direction (explore 2026-07-16) was explicit: home location by default, a travel mode for other locations, with already-scheduled trainings following automatically. A dated location log makes that resolution pure: heat forecasts for planned sessions during a booked training camp resolve to the camp's coordinates the moment the period is logged, because everything downstream is compute-on-read.

## What Changes

- New `location_periods` table (migration, next free slot): `{start_date, end_date, name, lat, lon, note?}` — inclusive date ranges, overlaps allowed with latest-`start_date` winning (the established macrocycle/public-feed resolution rule).
- New `internal/locations/` capability package: `POST /api/v1/locations` (Idempotency-Key supported), `GET /locations?from=&to=` (ascending window), `GET /locations/{id}`, `DELETE /locations/{id}` — no PATCH; corrections are delete + re-log (coach-memory precedent). Validation: `lat`/`lon` required and range-checked, `name` required, `date range` sane.
- **Home location config**: `HOME_LAT`/`HOME_LON` env pair (both-or-neither, the `WEB_USER`/`WEB_PASSWORD` gating pattern), exposed to consumers through a single resolution primitive: `LocationOn(date)` → covering travel period, else home, else "unconfigured".
- `GET /api/v1/locations/resolve?date=` — the resolution made visible (returns the effective location + its source: `travel`/`home`, or `404 location_unconfigured`).
- MCP tools: `log_location_period` (write — "I'm in Mallorca July 20–28"; the coach supplies coordinates until the weather change adds geocoding) + `list_location_periods` (read).
- `location_periods` classified **export-included** (user-authored, small).

## Capabilities

### New Capabilities

- `location-periods`: the dated location log, home fallback, resolution semantics, and MCP tools.

### Modified Capabilities

_None._

## Impact

- **Code:** migration; `internal/locations/` per the capability template; config additions (validated pair); dataexport classification; MCP + golden additive; `task swag`.
- **Privacy:** coordinates never leave the system except as weather-API query parameters (next change); the public feed and race page carry nothing location-derived.
- **Out of scope (deferred):** place-name geocoding on write (arrives with the weather client as an optional `place` parameter), auto-detection from Garmin activity locations (GPS is a standing non-goal), open-ended periods (end date required v1 — extend by re-logging).
