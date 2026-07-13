# coach-dashboard Specification (delta)

## ADDED Requirements

### Requirement: The stats surface presents a Performance Management Chart

The dashboard SHALL present a Performance Management Chart on the stats surface
(the `/stats` route or a sub-route), reading `GET /api/v1/performance/pmc`. The
chart SHALL render the CTL and ATL daily series as lines and TSB as an
area/bars around a zero baseline, over a selectable window (at least 90-day,
180-day, and 365-day options), and SHALL visually flag the weeks reported in
`ramp_alerts` on the CTL trace. The view SHALL reuse the existing analyst
visual language (visx, `Panel` / `Stat`, muted idiom — no celebratory
treatment) and SHALL degrade to an empty-state when the window has no training
history rather than erroring. The home route's training composite is unchanged
(the PMC introduces no new fetch there).

#### Scenario: The PMC renders for the selected window

- **WHEN** the user opens the stats surface and `GET /api/v1/performance/pmc`
  returns a populated series
- **THEN** the chart plots the CTL line, the ATL line, and TSB as an area/bars
  around a zero baseline for the selected window

#### Scenario: Window selector re-queries the series

- **WHEN** the user switches the window selector between 90, 180, and 365 days
- **THEN** the view requests the corresponding date range and re-renders the
  chart

#### Scenario: Ramp-alert weeks are flagged

- **WHEN** the response's `ramp_alerts` names one or more weeks
- **THEN** those weeks are visually highlighted on the chart

#### Scenario: Empty history degrades gracefully

- **WHEN** the selected window has no training history (an all-zero series)
- **THEN** the PMC panel shows an empty-state without erroring
- **AND** the home route (`/`) is unaffected
