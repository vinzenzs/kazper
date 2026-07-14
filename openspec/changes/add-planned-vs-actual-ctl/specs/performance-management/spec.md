## ADDED Requirements

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
