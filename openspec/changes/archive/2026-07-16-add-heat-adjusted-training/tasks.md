# Tasks

## 1. Weather client

- [x] 1.1 `internal/weather/`: Open-Meteo forecast/archive/geocode client (5 s timeout, TTL cache 30 min/process-life, fail-open `(data, ok)` shape); unit tests against a fake HTTP server (happy, timeout, malformed, cache-hit)
      _Beyond the list: unreachable-host, cancelled-context, ragged parallel arrays, unparseable timestamp, rate-limited, cache-expiry (injectable clock, no sleeps), archive-never-expires, and a geocode distinction the spec implies but doesn't name — **"matched nothing" (ok, empty) vs "the lookup is down" (not ok)**, which the locations write turns into `place_not_found` vs `geocoding_unavailable`. Wind is requested in m/s so nothing downstream re-converts Open-Meteo's km/h default._

## 2. Heat computation

- [x] 2.1 `internal/heat/` pure math: heat index + wind/cloud nudges → `heat_load_c`; acclimatization counter (outdoor completed, ≥60 min, HI ≥ 25 °C, 14 d); the adjustment table (heat load × duration × acclimatization); unit tests over every band boundary + hand-computed fixtures
      _Heat index is the NWS Rothfusz regression with its edge adjustments, falling back to the Steadman form below its valid floor (pinned against a published NWS table cell). Acclimatization: a missing **humidity** falls back to a neutral 50% rather than dropping the session (temperature dominates; discarding a genuinely hot ride over a missing field would under-report adaptation), while a missing **temperature** cannot qualify — we don't know how hot it was._
- [x] 2.2 `GET /workouts/{id}/heat` handler over narrow interfaces (planned workout, completed outdoor temps, effective config, sweat rate, `LocationOn`): indoor `not_applicable`, `assumed_outdoor`, `location_unconfigured` / `weather_unavailable` degradations, `workout_not_planned` 409
      _**Fluid-source deviation, deliberate.** The spec says "scaled from the measured sweat rate when derivable". Nothing is: `add-sweat-rate` made the field test require explicit pre/post weights BY DESIGN ("inferring pre/post would be a guess dressed as data") and persists no result, so there is no stored measurement to read. Garmin's per-workout `sweat_loss_ml` **is** stored and is real device data, so the fluid note is derived from it across recent long outdoor sessions — under its own source label **`garmin_sweat_loss_estimate`**, never `measured_sweat_rate`, and the note says "device estimate, not a field test". Labelling it "measured" would have contradicted the sweat-rate capability's own decision. No signal at all still yields the flagged `generic_default`._
      _A run's suggested pace is DIVIDED, not multiplied: backing off means more sec/km. Tested — the inverted axis is the kind of thing that silently ships backwards._
- [x] 2.3 Integration tests (fake weather backend): hot-day happy path, all degradations, indoor/null environments, acclimatization evidence echo, read-only
      _Also pinned: an indoor session costs **zero** forecast calls, `location_unconfigured` likewise; a logged travel period moves the forecast (and works with home unconfigured); indoor history never counts as acclimatization however hot the room; sessions outside the 14-day window don't count; identical weather + less adaptation → a deeper cut; a cool day suggests nothing; a missing baseline still yields a percentage; the forecast is cached across reads._
- [x] 2.4 `task swag`

## 3. Locations geocoding sugar

- [x] 3.1 Optional `place` on the locations POST + MCP tool (geocode via the client; `place_not_found` / `geocoding_unavailable`); tests
      _`place` also fills `name` when omitted, so "log Mallorca July 20–28" needs nothing else; an explicit name is never overwritten. Explicit `lat`/`lon` win and cost no lookup. Both failure paths refuse the write rather than storing a period without real coordinates — one would silently resolve every forecast to the wrong city. The MCP tool's golden entry is an **update, not an addition** (it gained `place`, `name`/`lat`/`lon` became optional) — an intended, spec'd change, the `pmc_series` sport-arg precedent._

## 4. Context & MCP

- [x] 4.1 `/context/daily` `heat` block (today + tomorrow planned outdoor; omission rules) + tests
      _Best-effort like the fuel block: a heat failure omits the block rather than failing the check-in bundle. Indoor, absent and degraded reports all yield no entry — the block exists to say "this needs attention", so a day with nothing to say has no key._
- [x] 4.2 `workout_heat` read tool (heuristic + advisory framing); golden regen (additive) + registry/integration green

## 5. Verification

- [x] 5.1 `task vet` + full suite green
- [ ] 5.2 Live: heat read for a real upcoming outdoor session against the actual forecast; sanity-check load and suggestion against the bloxi calculator for the same inputs
      _(operator step — needs real upcoming sessions and live Open-Meteo; not runnable in-session. **`HOME_LAT`/`HOME_LON` must be set in the deployment env** or every heat read degrades to `location_unconfigured`. Note this is the first outbound egress on a read path — a restrictive egress policy would show up as `weather_unavailable`.)_
