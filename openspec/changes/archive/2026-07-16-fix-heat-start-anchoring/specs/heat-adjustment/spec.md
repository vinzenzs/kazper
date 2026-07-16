## MODIFIED Requirements

### Requirement: A planned workout's heat load and suggested adjustment are computed on read

The system SHALL expose `GET /api/v1/workouts/{id}/heat` for a **planned** workout: resolve the
session date's location via the location-periods primitive (resolved name and source echoed;
unresolvable → `200` with `reason: "location_unconfigured"`), anchor the forecast window, fetch
the window's forecast (failure → `reason: "weather_unavailable"`), and compute: **`heat_load_c`** — a
composite °C-equivalent from the heat index (temperature × humidity) with bounded wind-cooling
and cloud-cover reductions (fixed documented constants); an **acclimatization level**
(`low` < 2, `medium` 2–4, `good` ≥ 5 qualifying sessions: outdoor completed, ≥ 60 min, session
heat index ≥ 25 °C, trailing 14 days — count and qualifying workout ids echoed); and a
**suggested adjustment**: a percentage reduction off the effective baseline (FTP-anchored for
bike, threshold-pace for run) from a fixed heat-load × duration-band × acclimatization table
printed in full in the response documentation, plus a fluid note scaled from the measured sweat
rate when derivable (generic guidance flagged otherwise).

**Window anchoring:** the forecast window SHALL span the session's duration from an anchor
chosen by precedence — an optional `start=HH:MM` query parameter (local time,
`400 start_invalid` on malformed values); else the workout's `started_at` when its time-of-day
is not exactly midnight; else the configured `DEFAULT_TRAINING_START` (validated `HH:MM`,
default `06:00`), re-anchored on the session's local calendar date and preserving its duration.
Exact midnight is the date-only scheduling path's sentinel for "no start time was given", never
a real start. The sentinel SHALL be recognised at exact midnight **in UTC or in the athlete's
timezone**: the scheduling path parses a date-only string and therefore stores *UTC* midnight,
which reads as a non-midnight local hour outside UTC — a local-only test would miss precisely
the rows this exists to correct. The response SHALL echo the effective `window` and
`start_source: "param" | "workout" | "assumed"`, with `assumed_start` carrying the applied
default (the `HH:MM` itself, so a wrong default is self-evident) when assumed. The
daily-context heat block SHALL use the same precedence (without the parameter) and carry
`assumed_start` when it applied.

A planned `indoor` session SHALL return `200 {not_applicable: true}` without fetching weather;
a null environment SHALL compute with `assumed_outdoor: true`; a non-planned workout SHALL
return `409 workout_not_planned`. The read SHALL be compute-on-read, persist nothing, and write
no targets anywhere — it is advisory input to the coach's existing confirmed update flows.

#### Scenario: A hot-day session gets load, level, and suggestion

- **WHEN** tomorrow's 2-hour outdoor ride forecasts 31 °C / 55 % humidity, and 6 qualifying
  hot outdoor sessions exist in the trailing 14 days
- **THEN** the response reports the composite `heat_load_c`, `acclimatization: "good"` with its
  evidence, a suggested percentage off effective FTP, and the fluid note

#### Scenario: A midnight-anchored scheduled session assumes the default start

- **WHEN** the planned workout's `started_at` is exactly midnight — in UTC (what the date-only
  scheduling path stores) or in the athlete's timezone — and no `start` parameter is supplied
- **THEN** the window anchors at `DEFAULT_TRAINING_START` on that local date, `start_source` is
  `"assumed"`, and `assumed_start` echoes the applied time

#### Scenario: A what-if start overrides the anchor

- **WHEN** the read is called with `start=10:00` for the same session
- **THEN** the window anchors at 10:00 (`start_source: "param"`) and the heat load reflects the
  late-morning forecast

#### Scenario: A real stored time anchors exactly

- **WHEN** the planned workout's `started_at` carries 06:30 local
- **THEN** the window spans 06:30 plus the duration with `start_source: "workout"` and no
  assumption fields

#### Scenario: Indoor sessions are not applicable

- **WHEN** the planned workout is marked `indoor`
- **THEN** the response is `200` with `not_applicable: true` and no weather fetch occurs

#### Scenario: Unknown environment is assumed, visibly

- **WHEN** the planned workout has a null environment
- **THEN** the heat computation runs with `assumed_outdoor: true` in the response

#### Scenario: Weather trouble degrades honestly

- **WHEN** the forecast fetch fails
- **THEN** the response is `200` with `reason: "weather_unavailable"` and no adjustment
