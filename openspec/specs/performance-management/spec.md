# performance-management Specification

## Purpose
TBD - created by archiving change add-performance-management. Update Purpose after archive.
## Requirements
### Requirement: GET /performance/pmc returns a daily CTL/ATL/TSB series over a date window

The system SHALL expose `GET /performance/pmc?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>&tz=<iana>`
returning one entry per calendar day in the inclusive `[from, to]` window,
ordered by date ascending. Each day SHALL carry `date`, `tss_total` (the sum of
`tss` over completed workouts whose `started_at` falls on that local day in
`tz`; `0` when none), `ctl`, `atl`, `tsb`, and `ramp_rate`. The series SHALL be
computed with the classic Coggan recurrences using fixed time constants:

```
ctl(d) = ctl(d−1) + (tss(d) − ctl(d−1)) / 42
atl(d) = atl(d−1) + (tss(d) − atl(d−1)) / 7
tsb(d) = ctl(d−1) − atl(d−1)
```

`tz` SHALL default to the server's `DEFAULT_USER_TZ` when omitted and be echoed
in the response. The response SHALL wrap the series as
`{from, to, tz, seed_date, days: [...], ramp_alerts: [...], missing_tss_workouts}`.

#### Scenario: Daily series renders over a populated window

- **WHEN** the client calls `GET /performance/pmc?from=2026-06-01&to=2026-06-30&tz=Europe/Berlin`
  and completed workouts with TSS exist in and before that window
- **THEN** the response contains exactly 30 `days` entries ordered by `date`
  ascending, each carrying `date`, `tss_total`, `ctl`, `atl`, `tsb`, and
  `ramp_rate`
- **AND** each day's `ctl` and `atl` follow the 42-day and 7-day EWMA
  recurrences from the previous day's values

#### Scenario: TSB reflects yesterday's fitness and fatigue

- **WHEN** day `d` has `ctl(d−1) = 60.0` and `atl(d−1) = 75.0`, and a
  200-TSS race is completed on day `d`
- **THEN** day `d`'s `tsb` is `-15.0` (computed from day `d−1`, unaffected by
  day `d`'s own TSS)

#### Scenario: A rest day decays the series rather than breaking it

- **WHEN** a calendar day in the window has no completed workouts
- **THEN** that day appears in `days` with `tss_total: 0`
- **AND** its `ctl` and `atl` decay per the recurrences (no gap, no null)

#### Scenario: Default tz falls back to DEFAULT_USER_TZ

- **WHEN** the client omits the `tz` query parameter
- **THEN** the day buckets are computed in the server's configured
  `DEFAULT_USER_TZ`
- **AND** the response echoes that `tz` value

#### Scenario: Numeric outputs are rounded at the response boundary

- **WHEN** any computed `tss_total`, `ctl`, `atl`, `tsb`, `ramp_rate`, or
  `ctl_delta` value would carry more than one decimal place
- **THEN** the response shows it rounded to one decimal place via the existing
  `numfmt` boundary rule
- **AND** the EWMA recurrence itself is carried at full precision internally
  (rounding is serialization-only)

### Requirement: The PMC series is seeded from the earliest completed workout

The system SHALL compute the EWMA series starting from a seed date — the local
date of the earliest completed workout minus one day — with `ctl = atl = 0` on
the seed date, iterating forward over every calendar day through `to`, and
returning only `[from, to]`. The response SHALL report `seed_date` so consumers
can judge warm-up coverage. The returned values SHALL therefore be independent
of the requested window: the same date returns the same `ctl`/`atl`/`tsb`
regardless of `from`. When no completed workouts exist at all, the endpoint
SHALL return an all-zero series for the window with `200 OK` and omit (or null)
`seed_date`.

#### Scenario: Values are window-independent

- **WHEN** the same date `2026-06-15` is fetched once via a 7-day window and
  once via a 180-day window
- **THEN** both responses report identical `ctl`, `atl`, and `tsb` for
  `2026-06-15` (the warm-up always runs from the seed date, not from `from`)

#### Scenario: Days before the first workout carry zeros

- **WHEN** `from` predates the earliest completed workout's local date
- **THEN** the days before that date appear with `tss_total: 0`, `ctl: 0`,
  `atl: 0`, `tsb: 0`

#### Scenario: Empty history degrades to an all-zero series

- **WHEN** the database contains no completed workouts
- **THEN** the response is `200 OK` with one all-zero entry per day in the
  window and no `seed_date` value
- **AND** `ramp_alerts` is an empty array

### Requirement: Only completed workouts contribute and missing TSS is surfaced, not imputed

The system SHALL sum `tss` over `status = 'completed'` workouts only; planned
workouts SHALL be excluded entirely. A completed workout whose `tss` is NULL
SHALL contribute `0` to its day's `tss_total` and SHALL be counted: each day
carries `missing_tss_count` (the number of completed workouts on that local day
with NULL `tss`), omitted when zero, and the response carries a window-level
`missing_tss_workouts` total over `[from, to]`. Workouts SHALL be bucketed by
the local date of `started_at` in the requested `tz` (start-day attribution — a
midnight-spanning workout belongs to the day it started).

#### Scenario: Planned workouts never contribute load

- **WHEN** a local day has one completed ride with `tss: 80` and one planned
  session carrying a planned TSS
- **THEN** that day's `tss_total` is `80.0`
- **AND** the planned session is not counted in `missing_tss_count` either

#### Scenario: A completed workout without TSS is counted, not hidden

- **WHEN** a local day has a completed ride with `tss: 80` and a completed
  strength session with `tss` NULL
- **THEN** that day's `tss_total` is `80.0` and its `missing_tss_count` is `1`
- **AND** the window-level `missing_tss_workouts` includes that session

#### Scenario: Days with full TSS coverage omit the counter

- **WHEN** every completed workout on a local day carries a non-null `tss`
- **THEN** that day's entry omits the `missing_tss_count` key (omitempty)

#### Scenario: Bucketing respects the requested timezone

- **WHEN** a workout starts at `2026-06-07T22:30:00Z` and `tz=Europe/Berlin`
  (local time 00:30 on June 8)
- **THEN** its TSS contributes to the `2026-06-08` day, not `2026-06-07`

### Requirement: Ramp rate is reported per day and unsafe weekly ramps are flagged

The system SHALL report on each day a `ramp_rate` equal to `ctl(d) − ctl(d−7)`
(CTL change per week; days before the seed evaluate as `ctl = 0`). The response
SHALL additionally carry a `ramp_alerts` array — always present, empty when
none — with one entry per Monday-start calendar week whose last day falls
inside `[from, to]` and whose CTL rise over that week exceeds the fixed safe
threshold of `8.0` CTL/week. Each alert SHALL carry `week_start`, `ctl_start`,
`ctl_end`, and `ctl_delta`.

#### Scenario: A too-fast build week is flagged

- **WHEN** a Monday-start week inside the window ends with CTL `9.4` higher
  than the day before the week began
- **THEN** `ramp_alerts` contains an entry for that week with `week_start`,
  `ctl_start`, `ctl_end`, and `ctl_delta: 9.4`

#### Scenario: A safe build produces no alerts

- **WHEN** every week in the window rises by `8.0` CTL or less (or falls)
- **THEN** `ramp_alerts` is `[]`
- **AND** per-day `ramp_rate` values are still present on every day

#### Scenario: Ramp rate near the seed uses the zero baseline

- **WHEN** a day within the first week after `seed_date` is returned
- **THEN** its `ramp_rate` equals its `ctl` (the `d−7` reference is `0`)

### Requirement: Window validation matches the workout-stats vocabulary

The system SHALL validate the window with the established error codes: missing
`from` or `to` returns `400 {"error":"range_required"}`; an unparseable date
returns `400 {"error":"date_invalid"}`; `from` after `to` returns
`400 {"error":"range_invalid"}`; a window spanning more than 400 days returns
`400 {"error":"range_too_large","max_days":400}`; an unknown IANA `tz` returns
`400 {"error":"tz_invalid"}`.

#### Scenario: Year-to-date is a supported window

- **WHEN** the requested window spans up to 400 days
- **THEN** the request succeeds with one entry per day

#### Scenario: Invalid windows are rejected with the matching code

- **WHEN** `from` is missing, a date is malformed, `from > to`, the span
  exceeds 400 days, or `tz` is not a valid IANA zone
- **THEN** the endpoint returns `400` with `range_required`, `date_invalid`,
  `range_invalid`, `range_too_large` (with `max_days: 400`), or `tz_invalid`
  respectively

### Requirement: The PMC endpoint is read-only and unit-isolated

The endpoint SHALL be read-only: it SHALL NOT write any rows and SHALL NOT
consume an `Idempotency-Key` header. CTL/ATL/TSB SHALL appear only in this
capability's response shape — never merged into nutrition, hydration, energy,
or fitness-metrics totals, and never written into the `fitness_metrics`
acute/chronic mirror (which remains Garmin's own, distinct load metric).

#### Scenario: The endpoint never writes

- **WHEN** the client invokes `GET /performance/pmc` any number of times, with
  or without an `Idempotency-Key` header
- **THEN** no rows are created, updated, or deleted, and no idempotency-cache
  row is recorded

#### Scenario: PMC values stay in their own shape

- **WHEN** the client fetches any other capability's summary or context
  response
- **THEN** no `ctl`, `atl`, or `tsb` field appears there, and the PMC response
  carries no nutrition, hydration, or Garmin-load fields

### Requirement: A target CTL trajectory is simulated from the macrocycle's declared phase targets

The system SHALL expose `GET /api/v1/performance/pmc/target-trajectory?macrocycle_id=&tz=`
returning, for each day of the resolved macrocycle's `[start_date, end_date]`, the target CTL
implied by the declared plan — daily target TSS = the containing phase's `target_weekly_tss / 7`,
folded through the same 42-day EWMA as the measured PMC, **seeded from the actual CTL on the
macrocycle start date** (echoed as `seed_ctl`) — beside the measured `actual_ctl` and `delta` for
days up to today (future days carry the target only). Days in no phase or in a phase without a
declared target SHALL simulate at 0 target TSS and be flagged `target_declared: false`. When
`macrocycle_id` is omitted the system SHALL resolve the active macrocycle (containing today,
latest `start_date` tie-break); an unknown id or no active macrocycle SHALL return
`404 macrocycle_not_found`. When no phase in the macrocycle declares a target the response SHALL
be `200` with `trajectory: null` and `reason: "targets_missing"`. The response SHALL carry a
summary with `current_delta`, `delta_trend_14d`, `projected_end_ctl_planned`, and
`projected_end_ctl_current` (the EWMA extended from today's actual CTL over the remaining planned
targets). Values SHALL be full-precision internally and rounded to 1 decimal at the boundary;
missing-TSS days SHALL be surfaced per the existing PMC convention, never imputed. The endpoint
SHALL be compute-on-read, persist nothing, and use the PMC error vocabulary for `tz`.

#### Scenario: An active macrocycle with targets returns both curves and deltas

- **WHEN** the endpoint is called with no `macrocycle_id` while an active macrocycle's phases
  declare `target_weekly_tss`
- **THEN** the response carries the full-span daily series with `target_ctl` throughout,
  `actual_ctl`/`delta` up to today, `seed_ctl`, and the four summary fields

#### Scenario: Undeclared spans decay and are flagged

- **WHEN** the macrocycle has a gap between phases or a phase with a null target
- **THEN** those days simulate at 0 target TSS with `target_declared: false` and the target CTL
  decays through them

#### Scenario: A macrocycle with no targets degrades with a reason

- **WHEN** no phase in the resolved macrocycle declares `target_weekly_tss`
- **THEN** the response is `200` with `trajectory: null` and `reason: "targets_missing"`

#### Scenario: No resolvable macrocycle is a 404

- **WHEN** no macrocycle contains today and no `macrocycle_id` is supplied, or the supplied id is
  unknown
- **THEN** the response is `404` with `macrocycle_not_found`

### Requirement: The target trajectory is readable over MCP

The system SHALL expose a `pmc_target_trajectory` MCP tool (read tier) issuing a single
`GET /api/v1/performance/pmc/target-trajectory` and forwarding the body verbatim, with optional
`macrocycle_id` and `tz` args. The tool description SHALL distinguish it from `pmc_series`
(measured load only) and state the active-macrocycle default.

#### Scenario: Agent reads plan-vs-actual in one call

- **WHEN** the agent invokes `pmc_target_trajectory` with no arguments during an active
  macrocycle
- **THEN** the tool issues one GET and returns the trajectory and summary verbatim

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

