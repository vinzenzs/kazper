## Context

Weather needs (lat, lon, date). Kazper stores no GPS (standing non-goal) and shouldn't — heat planning needs city-grade location, not tracks. The athlete trains from home except during travel; races carry their own location (handled in the race-weather change). The user chose a dedicated capability for the travel layer.

## Goals / Non-Goals

**Goals:** a pure, date-keyed resolution — `LocationOn(date)` — that every weather consumer shares; conversational entry ("Mallorca, July 20–28"); scheduled sessions follow a logged trip automatically because nothing is precomputed.

**Non-Goals:** GPS/auto-detection; geocoding (next change adds it as sugar on the write); per-workout location overrides (a period covers the case); open-ended "moved house" periods — that's a `HOME_LAT/LON` config change, not a travel row.

## Decisions

### D1 — Dated ranges, overlap tolerated, latest start wins
`{start_date, end_date}` inclusive; resolution picks the covering period with the latest `start_date` (macrocycle/public-feed rule — a weekend trip inside a training camp resolves to the weekend trip). No uniqueness constraint machinery; the tie-break is deterministic and auditable via `/locations/resolve`.

### D2 — Home is config, not a row
Home is quasi-static infrastructure (like `DEFAULT_USER_TZ`), validated as a pair at boot (`ErrHomeLocationIncomplete` on one-sided config — the WEB_USER pattern). Unconfigured home + no covering period = `location_unconfigured`, and weather consumers degrade with that reason rather than guessing. Moving house = change the env, not the log.

### D3 — Coordinates on the write, geocoding later
`lat` ∈ [-90, 90], `lon` ∈ [-180, 180], both required (`400 lat_lon_invalid`); `name` required so the log reads like a travel diary. The coach can supply city coordinates today; the weather change adds an optional `place` parameter that geocodes server-side (Open-Meteo) — additive, no shape break.

### D4 — No PATCH
Dated, small, coach-written rows: delete + re-log (coach-memory precedent). Extending a trip = delete + re-log with the new end date.

### D5 — The resolve endpoint exists for auditability
`GET /locations/resolve?date=` returns `{lat, lon, name, source: travel|home}` — when a heat forecast surprises, one call shows which location produced it. Weather consumers use the same internal primitive, so the endpoint can never disagree with behavior.

## Risks / Trade-offs

- **Forgotten travel logs** → forecasts quietly use home. Accepted: the coach asks about upcoming travel in check-ins, and the heat read echoes the resolved location name so a wrong city is visible in every response.
- **Coordinates from the coach's knowledge** are city-grade — exactly the precision weather needs; geocoding later removes even that dependence.

## Migration Plan

Migration on the next free slot (verify head; two sibling proposals also carry migrations — coordinate). Down drops the table. Export-included classification in the same change.

## Open Questions

- Elevation as an optional column (heat + altitude interact; Open-Meteo returns elevation per coordinate anyway) — defer until the heat model wants it.
