# workout-stats Specification (delta)

## ADDED Requirements

### Requirement: Time in zone is aggregated over a date range into window, by-sport, and weekly distributions

The system SHALL expose `GET /api/v1/workouts/intensity-distribution?from=&to=&tz=`
aggregating HR-zone time (`secs_in_zone_1..5`) over **completed** workouts whose
`started_at` falls in the inclusive `[from, to]` window, bucketed by local day in the
requested `tz` (start-day attribution). The response SHALL carry a window `total`, a
`by_sport` breakdown, and a Monday-start `weekly` trend. The `total` and each `by_sport`
and `weekly` entry SHALL carry `workouts_counted`, `total_zone_secs`, and a `zones` array
of exactly five entries ordered zone 1→5, each with `zone`, `secs` (integer sum), and
`share_pct` (`secs / total_zone_secs × 100`, computed at full precision and rounded at the
response boundary via `numfmt`).

A workout counts toward the sums when at least one `secs_in_zone_*` is non-null; a null
individual zone SHALL contribute nothing (present-only summing). Planned workouts SHALL be
excluded entirely. `by_sport` SHALL group by sport with a multisport workout contributing a
single `multisport` entry (no per-leg zone attribution). A `weekly` bucket SHALL be emitted
only for a Monday-start calendar week (in `tz`) containing at least one completed workout;
empty weeks SHALL NOT be zero-filled, and edge weeks SHALL aggregate only in-window days.
When the window has no zone time at all, zone entries SHALL carry `secs: 0` and omit
`share_pct`, and the response SHALL still be `200 OK`.

`from` and `to` are `YYYY-MM-DD`; `tz` SHALL default to the server's `DEFAULT_USER_TZ` and
be echoed. Validation SHALL reuse the capability's vocabulary: missing `from`/`to` returns
`400 range_required`; an unparseable date `400 date_invalid`; `from` after `to`
`400 range_invalid`; a window over 400 days `400 range_too_large` with `max_days`; an
invalid `tz` `400 tz_invalid`. The endpoint SHALL be read-only and SHALL NOT consume an
`Idempotency-Key`. Zone-time seconds and shares SHALL appear only in this response shape
and SHALL NOT be merged into any nutrition, hydration, energy, or volume total.

#### Scenario: Window distribution renders with shares per zone

- **WHEN** `GET /api/v1/workouts/intensity-distribution?from=2026-05-04&to=2026-06-28&tz=Europe/Berlin`
  is requested and completed workouts with zone data exist in the range
- **THEN** `total` carries `workouts_counted`, `total_zone_secs`, and five `zones` entries
  ordered zone 1→5, each with `secs` and a `share_pct` that sums (within rounding) to 100

#### Scenario: By-sport and weekly breakdowns use the same shape

- **WHEN** the window contains zone-bearing runs and rides across two Monday-start weeks
- **THEN** `by_sport` has a `run` and a `bike` entry and `weekly` has one bucket per week
  with `week_start`, each carrying the same `workouts_counted` / `total_zone_secs` /
  `zones` shape as `total`

#### Scenario: Planned workouts and null zones contribute nothing

- **WHEN** the range contains a planned workout with zone estimates absent and a completed
  workout with `secs_in_zone_5` null but other zones set
- **THEN** the planned workout is excluded entirely and the completed workout contributes
  its non-null zones only (zone 5 gains nothing, the sums are not zeroed)

#### Scenario: Weekly buckets skip empty weeks

- **WHEN** the window spans four Monday-start weeks and only weeks one and three contain
  completed workouts
- **THEN** `weekly` contains exactly two buckets (weeks one and three) and no zero-filled
  buckets for the empty weeks

#### Scenario: Bucketing respects the requested timezone

- **WHEN** a workout starts at `2026-06-07T22:30:00Z` and `tz=Europe/Berlin` (local time
  00:30 on June 8, a Monday)
- **THEN** its zone time contributes to the week starting `2026-06-08`, not the prior week

#### Scenario: A zone-less window degrades honestly

- **WHEN** the window contains no completed workouts with zone data
- **THEN** the response is `200 OK` with `total.total_zone_secs: 0`, five `zones` entries
  with `secs: 0` and no `share_pct`, and an empty `weekly` array or buckets carrying only
  missing-data counts

#### Scenario: Invalid windows are rejected with the matching code

- **WHEN** `from`/`to` is missing, a date is unparseable, `from` is after `to`, the window
  exceeds 400 days, or `tz` is invalid
- **THEN** the endpoint returns `400` with `range_required`, `date_invalid`,
  `range_invalid`, `range_too_large` (with `max_days`), or `tz_invalid` respectively

### Requirement: The window distribution carries an auditable polarization classification

The intensity-distribution response's `total` SHALL carry a `bands` object collapsing the
five zones into three shares — `low_pct` (zones 1+2), `moderate_pct` (zone 3), `high_pct`
(zones 4+5), computed from full-precision sums and rounded at the boundary — and a
`classification` label derived deterministically from those bands with fixed, documented
constants (`thresholdBandPct = 20.0`, `lowBaseBandPct = 75.0`): no zone time → null;
moderate share ≥ 20.0 → `threshold`; else low share ≥ 75.0 with high > moderate →
`polarized`; else low share ≥ 75.0 → `pyramidal`; else → `mixed`. Every window SHALL
receive exactly one label (or null). The classification SHALL be computed only at window
level — `weekly` and `by_sport` entries carry no label — and the `bands` SHALL always
accompany the label so consumers can audit it.

#### Scenario: A base-heavy window with hard work classifies as polarized

- **WHEN** the window's full-precision band shares are low 80.2%, moderate 7.8%, high 12.0%
- **THEN** `total.classification` is `polarized` and `total.bands` reports those shares
  rounded to one decimal place

#### Scenario: A big-middle window classifies as threshold

- **WHEN** the window's moderate (zone 3) share is 24.5%
- **THEN** `total.classification` is `threshold` regardless of the low/high split

#### Scenario: A base-heavy window without dominant high intensity classifies as pyramidal

- **WHEN** the window's band shares are low 79.0%, moderate 12.0%, high 9.0%
- **THEN** `total.classification` is `pyramidal`

#### Scenario: No zone time yields a null classification

- **WHEN** the window contains no zone-bearing completed workouts
- **THEN** `total.classification` is null and no label is invented

### Requirement: Workouts without zone data are excluded but counted, not hidden

A completed workout in the window whose `secs_in_zone_1..5` are all null SHALL be excluded
from every zone sum, share, band, and classification input, and SHALL be counted instead:
the response SHALL carry a window-level `missing_zone_data_count` (always present, `0` at
full coverage) and each `weekly` bucket SHALL carry a `missing_zone_data_count` omitted
when zero. `workouts_counted` plus the window `missing_zone_data_count` SHALL equal the
number of completed workouts in the window. A `weekly` bucket SHALL be emitted for a week
whose only completed workouts lack zone data, so the per-week honesty count cannot vanish.

#### Scenario: A zone-less strength session is counted, not summed

- **WHEN** a window week contains a completed ride with zone data and a completed strength
  session with all `secs_in_zone_*` null
- **THEN** the strength session contributes nothing to any zone sum, that week's bucket
  carries `missing_zone_data_count: 1`, and the window `missing_zone_data_count` includes it

#### Scenario: Full zone coverage reports a zero count and omits per-week counters

- **WHEN** every completed workout in the window carries zone data
- **THEN** the window `missing_zone_data_count` is `0` and no `weekly` bucket carries a
  `missing_zone_data_count` key

### Requirement: The distribution reports a session-count axis over training_focus

The intensity-distribution response SHALL carry `by_training_focus` — a map from stored
`training_focus` value (the 7-zone German classification) to the count of completed
workouts in the window annotated with that focus — and `unclassified_focus_count`, the
count of completed workouts with a null `training_focus`. This axis is session counts of
recorded intent, independent of the measured `secs_in_zone_*` sums: it SHALL NOT be
duration-weighted and SHALL NOT feed the `bands` or `classification`, and the system SHALL
NOT derive or infer `training_focus` from zone data.

#### Scenario: Focus counts render next to the zone distribution

- **WHEN** the window contains 16 completed workouts marked `basic_endurance_1`, 4 marked
  `development`, and 8 with no `training_focus`
- **THEN** `by_training_focus` maps `basic_endurance_1` to 16 and `development` to 4, and
  `unclassified_focus_count` is 8

#### Scenario: Focus intent never alters the measured classification

- **WHEN** every workout in the window is annotated `recovery` but the measured zone time
  is 30% in zone 3
- **THEN** `total.classification` is `threshold` (from the measured bands), unaffected by
  the annotations

### Requirement: The intensity distribution is readable over MCP

The system SHALL expose an `intensity_distribution` MCP tool, registered via the shared
`agenttools` registry, that issues a single `GET /api/v1/workouts/intensity-distribution`
and forwards the response verbatim, so the coaching agent can answer "have I been training
polarized?" without client-side aggregation. The tool SHALL accept `from` and `to`
(inclusive `YYYY-MM-DD`) and an optional `tz`, mirroring the REST query 1:1; it is
read-only and SHALL NOT send an `Idempotency-Key`. Its description SHALL name the
zone-share and classification semantics and point volume questions at `training_totals`.

#### Scenario: Agent reads the intensity distribution in one call

- **WHEN** the agent invokes `intensity_distribution` with a `from`, `to`, and optional `tz`
- **THEN** the tool issues exactly one `GET /workouts/intensity-distribution` request and
  returns the window, by-sport, and weekly distributions with the classification as the
  tool result

#### Scenario: The tool is announced from the registry

- **WHEN** a client calls `tools/list`
- **THEN** `intensity_distribution` appears in the announced surface, derived from its
  `agenttools` registry entry, and its announced input schema matches the regenerated
  golden baseline
