# Tasks

## 1. Weather client

- [ ] 1.1 `internal/weather/`: Open-Meteo forecast/archive/geocode client (5 s timeout, TTL cache 30 min/process-life, fail-open `(data, ok)` shape); unit tests against a fake HTTP server (happy, timeout, malformed, cache-hit)

## 2. Heat computation

- [ ] 2.1 `internal/heat/` pure math: heat index + wind/cloud nudges → `heat_load_c`; acclimatization counter (outdoor completed, ≥60 min, HI ≥ 25 °C, 14 d); the adjustment table (heat load × duration × acclimatization); unit tests over every band boundary + hand-computed fixtures
- [ ] 2.2 `GET /workouts/{id}/heat` handler over narrow interfaces (planned workout, completed outdoor temps, effective config, sweat rate, `LocationOn`): indoor `not_applicable`, `assumed_outdoor`, `location_unconfigured` / `weather_unavailable` degradations, `workout_not_planned` 409
- [ ] 2.3 Integration tests (fake weather backend): hot-day happy path, all degradations, indoor/null environments, acclimatization evidence echo, read-only
- [ ] 2.4 `task swag`

## 3. Locations geocoding sugar

- [ ] 3.1 Optional `place` on the locations POST + MCP tool (geocode via the client; `place_not_found` / `geocoding_unavailable`); tests

## 4. Context & MCP

- [ ] 4.1 `/context/daily` `heat` block (today + tomorrow planned outdoor; omission rules) + tests
- [ ] 4.2 `workout_heat` read tool (heuristic + advisory framing); golden regen (additive) + registry/integration green

## 5. Verification

- [ ] 5.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 5.2 Live: heat read for a real upcoming outdoor session against the actual forecast; sanity-check load and suggestion against the bloxi calculator for the same inputs
