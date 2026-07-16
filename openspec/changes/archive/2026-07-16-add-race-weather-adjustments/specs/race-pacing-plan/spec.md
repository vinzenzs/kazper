## ADDED Requirements

### Requirement: The pacing plan optionally annotates heat-adjusted bands

`GET /api/v1/races/{id}/pacing-plan` SHALL accept an optional `weather=true` parameter: the
race's `location` text is geocoded (resolved name echoed), the race window's forecast fetched,
and the shipped heat model applied — the response gains a race-level
`heat: {load_c, acclimatization, location, forecast_at}` block and, per computable leg, a
`heat_adjusted` sibling of the original band (band, IF, estimated TSS) derived by applying the
heat adjustment to that leg's targets. Original bands SHALL always remain present and
unchanged; without the parameter the response SHALL be byte-identical to today's. Degradations
keep the base plan intact: unmatchable location → `heat_reason: "location_ungeocodable"`; race
date beyond forecast range → `"forecast_out_of_range"`; weather failure →
`"weather_unavailable"`.

#### Scenario: A hot forecast annotates every leg

- **WHEN** the pacing plan is requested with `weather=true` inside forecast range and the race
  location geocodes
- **THEN** each leg carries `heat_adjusted` beside its original band and the race-level heat
  block names the load and location

#### Scenario: Without the flag nothing changes

- **WHEN** the plan is requested without `weather`
- **THEN** the response is identical to the pre-change contract

#### Scenario: An out-of-range race degrades to the base plan

- **WHEN** the race is 30 days out and `weather=true` is supplied
- **THEN** the original plan returns with `heat_reason: "forecast_out_of_range"` and no
  adjusted bands
