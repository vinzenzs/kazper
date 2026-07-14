## ADDED Requirements

### Requirement: The stats view shows a power-profile panel

The `/stats` dashboard view SHALL render a power-profile panel driven by
`GET /workouts/power-profile`: one row per present anchor (neuromuscular / anaerobic / VO₂max /
threshold) showing the duration, watts, W/kg, the Coggan category badge, and the interpolated
percentile, plus the rider `phenotype` label when present. Missing anchors SHALL be shown as
absent (not fabricated), and the panel SHALL render an empty/degraded state when the window has no
rankable efforts or when weight is unavailable, rather than an error.

#### Scenario: The panel renders the ranked anchors

- **WHEN** the power-profile read returns anchors with categories
- **THEN** the panel shows each anchor's W/kg, category badge and percentile, plus the phenotype
  label when all four anchors are present

#### Scenario: The panel degrades without an error

- **WHEN** the read reports `weight_data_missing` or returns no anchors
- **THEN** the panel shows a neutral empty/degraded state, not an error banner
