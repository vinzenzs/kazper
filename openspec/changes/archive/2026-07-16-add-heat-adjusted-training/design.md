## Context

Completed workouts carry `temperature_c`/`humidity_pct`/`wind_speed_mps` (bridge-synced) and, after `add-workout-environment`, an indoor/outdoor mark. `add-location-periods` resolves (lat, lon) per date. Effective FTP/threshold pace exist post threshold-split; measured sweat rate exists on demand. The bloxi calculator demonstrates the output shape: composite heat load (°C-equivalent) → % adjustment off a cool-weather baseline, openly a heuristic. Open-Meteo is keyless, free for non-commercial use, and offers forecast, historical archive, and geocoding — the OFF client is the in-repo precedent for a guarded outbound integration.

## Goals / Non-Goals

**Goals:** a pre-session heat read whose every input is echoed (location name, forecast values, acclimatization evidence, baseline, constants band); fail-open on the network; strictly advisory — the coach converts it into confirmed workout updates through existing flows.

**Non-Goals:** writing adjustments anywhere (no compiled-workout mutation, no target rewrites — user decision); WBGT claims (no solar sensor; the heuristic says it's a heuristic); race weather (next change); per-workout GPS.

## Decisions

### D1 — Weather client: keyless, cached, fail-open, its own capability
`internal/weather/`: `Forecast(lat, lon, from, to)` (hourly temp/humidity/wind/cloud), `Archive(...)` (same shape, past), `Geocode(place)` (top match + country). HTTP timeout ~5 s; in-memory TTL cache (30 min forecast, permanent-per-process archive); every consumer receives `(data, ok)` — a failure yields `weather_unavailable` in *their* response, never a 5xx. Capability-specced like `off-integration` so the contract (not the vendor SDK) is the documented surface.

### D2 — Heat load: bloxi-shaped composite, constants v1
Start from the session window's mean forecast: heat index (temperature × humidity, standard Rothfusz-style regression), plus fixed nudges — wind above ~3 m/s subtracts (convective cooling, capped), cloud cover subtracts a bounded fraction of the solar penalty (cloud is the v1 solar proxy). Output labeled `heat_load_c` (°C-equivalent). Constants documented in the spec, echoed per response, and **refined on evidence** (user direction) — the eventual heat-analytics change supplies exactly that evidence.

### D3 — Acclimatization from data, not a dropdown
`good` ≥ 5, `medium` 2–4, `low` < 2 outdoor completed sessions of ≥ 60 min with session heat index ≥ 25 °C in the trailing 14 days (workout `temperature_c`/`humidity_pct`; environment `indoor` excluded, null counts with the assumed-outdoor caveat). The count and qualifying sessions are echoed — the level is auditable back to rides. Constants v1.

### D4 — Adjustment bands
% reduction off the effective baseline from a fixed 3-axis table: heat load (bands from ~24 °C up) × duration (<45 / 45–150 / >150 min) × acclimatization (good shaves the reduction, low deepens it), bike applied to FTP-anchored targets, run to threshold pace. The table is small, fully printed in the spec, and advisory: the response says "suggested", never "set". Fluid note: measured sweat rate × a heat-load multiplier when a recent field test exists, else the generic guidance flagged as such.

### D5 — The coach-update loop is context + existing writes
`/context/daily`'s `heat` block (today + tomorrow) is the standing trigger; a material overnight forecast change simply shows up in the next read. Applying = the coach proposes edits via existing template/plan/schedule tools with write-confirm — this change adds **zero** write surface.

### D6 — Indoor and unknown environments
Planned `indoor` → `200 {not_applicable: true}` (no forecast fetched). Null environment → computed with `assumed_outdoor: true` (the field's specced semantics). Completed workouts are rejected (`409 workout_not_planned`) — history belongs to the analytics change.

## Risks / Trade-offs

- **Heuristic mistaken for physiology** — mitigated by naming, echoed constants, and the bloxi-style disclaimer in the tool description.
- **Outbound dependency** — first network egress on the read path; mitigated by cache + fail-open + the OFF precedent (and Open-Meteo's generous free tier; self-hostable if it ever matters).
- **Acclimatization cold-start** (sparse outdoor history) → `low` with visible evidence; correct behavior, reads conservative.

## Migration Plan

None (in-memory cache only). Rollback = revert routes/tool/context fold.

## Open Questions

- Solar radiation as a real input (Open-Meteo serves it) once constants get their evidence pass.
- A "forecast changed materially since yesterday" delta flag in context — deferred; the coach re-reads daily anyway.
