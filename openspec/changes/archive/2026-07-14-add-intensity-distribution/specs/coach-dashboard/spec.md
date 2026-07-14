# coach-dashboard Specification (delta)

## ADDED Requirements

### Requirement: The stats surface presents an intensity-distribution panel

The dashboard SHALL present an intensity-distribution panel on the stats surface (the
`/stats` route or a sub-route), reading `GET /api/v1/workouts/intensity-distribution`. The
panel SHALL render the window total as a zone-share bar (zones 1–5) with the
`classification` label and band shares displayed alongside it, and the `weekly` series as
per-week stacked zone-share bars over a selectable window. A non-zero
`missing_zone_data_count` SHALL be surfaced as a muted honesty note next to the chart
rather than hidden. The view SHALL reuse the existing analyst visual language (visx,
`Panel` / `Stat`, muted idiom — no celebratory treatment) and SHALL degrade to an
empty-state when the window has no zone-bearing workouts rather than erroring. The home
route's training composite is unchanged (this panel introduces no new fetch there).

#### Scenario: The distribution panel renders for the selected window

- **WHEN** the user opens the stats surface and
  `GET /api/v1/workouts/intensity-distribution` returns a populated distribution
- **THEN** the panel shows the window zone-share bar with the classification label and band
  shares, and the per-week stacked zone-share bars

#### Scenario: Window selection re-queries the distribution

- **WHEN** the user changes the stats surface's period selection
- **THEN** the view requests the corresponding date range and re-renders the panel

#### Scenario: Missing zone data is surfaced, not hidden

- **WHEN** the response carries a non-zero `missing_zone_data_count`
- **THEN** the panel shows the excluded-workout count as a muted note

#### Scenario: A zone-less window degrades gracefully

- **WHEN** the selected window has no completed workouts with zone data
- **THEN** the panel shows an empty-state without erroring
- **AND** the home route (`/`) is unaffected
