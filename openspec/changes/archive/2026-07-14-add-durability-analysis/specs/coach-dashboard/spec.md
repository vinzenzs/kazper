## ADDED Requirements

### Requirement: The stats route renders a durability fade grid

The dashboard's `/stats` route SHALL render a durability panel fed by
`GET /api/v1/workouts/durability`: a durations × kJ-tiers grid of tiered best power with fade
percentage against the fresh best, using the stats window selector conventions. When the
endpoint reports `no_tiered_data` the panel SHALL render an explanatory empty state (naming the
stream recompute backfill) rather than an empty grid; a fetch failure SHALL leave the rest of
`/stats` unaffected.

#### Scenario: Tiered data renders as a fade grid

- **WHEN** the window carries fresh and tiered records
- **THEN** the panel shows per-duration rows with fresh watts and per-tier watts + fade

#### Scenario: No tiered data renders the explanatory empty state

- **WHEN** the endpoint returns `reason: "no_tiered_data"`
- **THEN** the panel explains that historical rides need recompute and renders no grid
