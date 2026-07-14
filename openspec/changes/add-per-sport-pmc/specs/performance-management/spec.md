## ADDED Requirements

### Requirement: The PMC is filterable by sport

`GET /api/v1/performance/pmc` SHALL accept an optional `sport=` parameter (the workouts sport
vocabulary; multisport is its own value, never split across segments) computing the identical
CTL/ATL/TSB EWMA over only that sport's completed-workout TSS — warm-up, `seed_date`, weekly
ramp alerts, and missing-TSS surfacing all derived within the filtered series. When `sport` is
omitted the combined behavior SHALL be unchanged; the response SHALL echo `sport` when filtered;
an unknown value SHALL return `400 sport_invalid`. The `pmc_series` MCP tool SHALL forward an
optional `sport` arg to the same effect, and its description SHALL note that the combined series
is not the sum of the filtered ones.

#### Scenario: A run-filtered PMC reflects only run load

- **WHEN** `GET /performance/pmc?from=&to=&sport=run` is requested
- **THEN** CTL/ATL/TSB derive from run workouts' TSS only, `seed_date` reflects the earliest run
  workout, and the response echoes `sport: "run"`

#### Scenario: Omitting sport preserves today's combined series

- **WHEN** the endpoint is called without `sport`
- **THEN** the response is identical to the pre-change combined behavior

#### Scenario: An unknown sport is rejected

- **WHEN** `sport=rowing` is supplied
- **THEN** the response is `400` with `sport_invalid`
