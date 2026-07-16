# heat-adjustment Specification

## Purpose

Answer, before a session rather than after it: how hot will this actually be, how heat-adapted
is the athlete right now, and how much should they back off? Heat measurably degrades
sustainable power and pace, and every input the practice calculators ask a coach to type in is
already in Kazper — baseline from the effective config, duration from the planned session,
location from location-periods, weather from the forecast, and adaptation from the athlete's own
recent outdoor work.

The capability is openly a **heuristic** and says so everywhere it can: this is not WBGT and not
physiology. There is no solar sensor, so cloud cover is the proxy; the constants are v1, printed
in the response, and meant to be refined once the heat-analytics evidence exists. Every number
is decomposed — the heat index, the solar penalty, the wind cooling, the qualifying sessions
behind an acclimatization level — so a surprising answer can be taken apart rather than argued
with, and the resolved location name is echoed so a forecast for the wrong city is visible at a
glance rather than buried.

Acclimatization is derived, never asked for: a dropdown records what the athlete believes, a
count of their genuinely hot, genuinely long recent sessions records what happened. Sparse
history reads `low` with the evidence shown — conservative, and correct.

It is **strictly advisory and writes nothing**. No compiled workout is mutated, no target
rewritten. The read is the input to a conversation; the coach proposes edits and the athlete
confirms them through the flows that already exist.
## Requirements
### Requirement: A planned workout's heat load and suggested adjustment are computed on read

The system SHALL expose `GET /api/v1/workouts/{id}/heat` for a **planned** workout: resolve the
session date's location via the location-periods primitive (resolved name and source echoed;
unresolvable → `200` with `reason: "location_unconfigured"`), fetch the session window's
forecast (failure → `reason: "weather_unavailable"`), and compute: **`heat_load_c`** — a
composite °C-equivalent from the heat index (temperature × humidity) with bounded wind-cooling
and cloud-cover reductions (fixed documented constants); an **acclimatization level**
(`low` < 2, `medium` 2–4, `good` ≥ 5 qualifying sessions: outdoor completed, ≥ 60 min, session
heat index ≥ 25 °C, trailing 14 days — count and qualifying workout ids echoed); and a
**suggested adjustment**: a percentage reduction off the effective baseline (FTP-anchored for
bike, threshold-pace for run) from a fixed heat-load × duration-band × acclimatization table
printed in full in the response documentation, plus a fluid note scaled from the measured sweat
rate when derivable (generic guidance flagged otherwise). A planned `indoor` session SHALL
return `200 {not_applicable: true}` without fetching weather; a null environment SHALL compute
with `assumed_outdoor: true`; a non-planned workout SHALL return `409 workout_not_planned`.
The read SHALL be compute-on-read, persist nothing, and write no targets anywhere — it is
advisory input to the coach's existing confirmed update flows.

#### Scenario: A hot-day session gets load, level, and suggestion

- **WHEN** tomorrow's 2-hour outdoor ride forecasts 31 °C / 55 % humidity, and 6 qualifying
  hot outdoor sessions exist in the trailing 14 days
- **THEN** the response reports the composite `heat_load_c`, `acclimatization: "good"` with its
  evidence, a suggested percentage off effective FTP, and the fluid note

#### Scenario: Indoor sessions are not applicable

- **WHEN** the planned workout is marked `indoor`
- **THEN** the response is `200` with `not_applicable: true` and no weather fetch occurs

#### Scenario: Unknown environment is assumed, visibly

- **WHEN** the planned workout has a null environment
- **THEN** the heat computation runs with `assumed_outdoor: true` in the response

#### Scenario: Weather trouble degrades honestly

- **WHEN** the forecast fetch fails
- **THEN** the response is `200` with `reason: "weather_unavailable"` and no adjustment

### Requirement: The heat read is available over MCP

The system SHALL expose a `workout_heat` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/heat` and forwarding the body verbatim. The description SHALL state
that the model is a practice heuristic (not WBGT), strictly advisory, and that applying an
adjustment means proposing edits to the scheduled workout through the existing confirmed flows.

#### Scenario: The agent reads tomorrow's heat picture in one call

- **WHEN** the agent invokes `workout_heat` for tomorrow's planned session
- **THEN** one GET is issued and the load, level, and suggestion return verbatim

### Requirement: Heat analytics quantify performance degradation across history

The system SHALL expose `GET /api/v1/workouts/heat-analytics?from=&to=&tz=` over outdoor
completed workouts carrying temperature data: sessions bucketed by session heat index
(< 20 / 20–25 / 25–30 / > 30 °C), each bucket reporting session count, mean duration, mean EF,
mean decoupling, and mean output (power or pace) relative to the window baseline; plus Spearman
correlations (EF vs heat index, decoupling vs heat index) gated at 10 pairs
(`insufficient_pairs` per metric below it — the wellness-correlation posture). Workouts with a
null environment SHALL count with an `assumed_outdoor` tally; indoor SHALL be excluded. The
read SHALL be compute-on-read, persist nothing, use the shared range vocabulary (400-day cap),
and exists explicitly as the evidence stream for refining the heat-adjustment constants.

#### Scenario: A season shows the degradation gradient

- **WHEN** the window holds outdoor sessions across all four buckets
- **THEN** each bucket reports its means and the correlations carry rho with their pair counts

#### Scenario: Thin heat exposure gates the correlation

- **WHEN** only 6 sessions exceed 25 °C heat index
- **THEN** bucket means still report and the correlations return `insufficient_pairs` where
  pairs fall short

### Requirement: Heat analytics are readable over MCP

The system SHALL expose a `heat_analytics` MCP tool (read tier, one GET, verbatim). The
description SHALL note the duration confound (hot sessions skew long) and that findings inform
proposed constant refinements, not automatic ones.

#### Scenario: The agent reads the heat evidence in one call

- **WHEN** the agent invokes `heat_analytics` over the season
- **THEN** one GET is issued and the buckets and correlations return verbatim

