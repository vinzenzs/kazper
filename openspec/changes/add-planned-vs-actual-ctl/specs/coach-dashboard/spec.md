## ADDED Requirements

### Requirement: The PMC panel overlays the target CTL trajectory

The dashboard's `/stats` PMC panel SHALL overlay the target CTL curve (dashed line, undeclared
spans rendered muted) from `GET /api/v1/performance/pmc/target-trajectory` when an active
macrocycle with declared targets exists, with a compact on-plan readout derived from
`summary.current_delta`. When the endpoint returns `404 macrocycle_not_found`,
`targets_missing`, or the fetch fails, the overlay and readout SHALL be absent and the measured
PMC panel SHALL render exactly as today.

#### Scenario: An active macrocycle with targets shows the overlay

- **WHEN** `/stats` loads during an active macrocycle whose phases declare targets
- **THEN** the PMC panel shows the dashed target line over the measured CTL and the
  on-plan/behind readout

#### Scenario: No plan data leaves the panel unchanged

- **WHEN** there is no active macrocycle or its phases declare no targets
- **THEN** the PMC panel renders its existing measured-only view with no overlay and no error
