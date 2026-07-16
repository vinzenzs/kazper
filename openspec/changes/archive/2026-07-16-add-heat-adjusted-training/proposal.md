## Why

The core of the weather arc (explore 2026-07-16, modeled on thebloxi.at/heatcalculator): heat measurably degrades sustainable power/pace, and every input the manual calculator asks for, Kazper already has or can fetch — baseline from the effective config, duration from the planned session, acclimatization **derivable from recent outdoor completed-workout temperatures** (the field the bridge already syncs), weather from Open-Meteo (the same keyless API the calculator uses), location from the new location-periods resolution. The user's direction: coach layer only — heat adjustments inform the morning conversation and the coach updates scheduled workouts through existing confirmed flows; nothing rewrites a compiled workout silently.

## What Changes

- **New `internal/weather/` client (capability `weather-integration`)** — Open-Meteo forecast + archive + geocoding over HTTPS, keyless: hourly temperature/humidity/wind/cloud for a (lat, lon, time window). Guarded and fail-open (the OFF-client posture): timeouts, in-memory TTL cache (~30 min forecast / immutable archive), any failure degrades consumers to `weather_unavailable`, never errors them.
- **`GET /api/v1/workouts/{id}/heat?carbs...` → `GET /workouts/{id}/heat`** for a **planned** workout: resolves location (`LocationOn(date)`, name echoed), fetches the session window's forecast, computes a composite **heat load** (°C-equivalent from temperature + humidity + wind + cloud, bloxi-style heuristic, constants v1), an **acclimatization level** (`low`/`medium`/`good` from outdoor completed sessions ≥ 60 min above a heat-index threshold in the trailing 14 days), and the **suggested adjustment**: % off the effective baseline (FTP for bike, threshold pace for run) banded by heat load × duration × acclimatization, plus a fluid note scaled from measured sweat rate when one is derivable. Indoor planned sessions return `not_applicable`; `environment: null` computes with `assumed_outdoor: true`.
- `/context/daily` gains a `heat` block — today's and tomorrow's planned-session heat loads + suggested adjustments — the trigger for the coach-updates-workouts loop (existing write-confirm flows; no new write machinery).
- `location-periods` write gains an optional `place` parameter, geocoded server-side (`400 place_not_found` on no match) — the deferred sugar.
- New `workout_heat` MCP tool (read tier, one GET, verbatim; description: heuristic, advisory, coach-layer only).
- Compute-on-read, no migration.

## Capabilities

### New Capabilities

- `weather-integration`: the Open-Meteo client contract — endpoints, caching, fail-open guarantees.
- `heat-adjustment`: heat load, acclimatization, the adjustment read, and its MCP tool.

### Modified Capabilities

- `location-periods`: 1 ADDED requirement — geocoded `place` writes.
- `daily-context`: 1 ADDED requirement — the `heat` block.

## Impact

- **Code:** `internal/weather/` (client, cache, guarded); `internal/heat/` (pure heat-load/adjustment math over narrow interfaces: planned workouts, completed outdoor temps, effective config, sweat rate, locations); context fold; MCP + golden; `task swag`. Second-ever outbound integration (after OFF) — same self-hosted-friendly posture (no key, no account).
- **Out of scope (deferred):** race-day weather (next change), auto-writing adjusted targets into compiled Garmin workouts (explicitly rejected — coach layer only), solar-radiation input (Open-Meteo provides it; the cloud-cover proxy is v1), heat-analytics over history (next change), refining constants (on evidence, per user direction).
