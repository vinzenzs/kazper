## ADDED Requirements

### Requirement: The stats surface presents a power/pace curve

The dashboard SHALL present a mean-maximal power/pace curve chart, reading
`GET /api/v1/workouts/power-curve`, on the stats surface (the `/stats` route or a sub-route). The
chart SHALL render best value against duration on a logarithmic duration axis, with a sport
selector (power for bike, pace for run/swim) and a window selector, in the existing analyst
visual language (visx, `Panel`/`Stat`, muted). The per-workout detail route (from Phase 1) MAY
overlay that activity's own curve on the all-time/windowed best. An empty window SHALL render an
empty-state rather than erroring.

#### Scenario: The curve renders for the selected sport and window

- **WHEN** the user opens the curve view and `GET /api/v1/workouts/power-curve` returns records
- **THEN** the chart plots best value against duration on a log axis for the selected sport and
  window

#### Scenario: Sport and window selectors re-query the curve

- **WHEN** the user changes the sport or window selector
- **THEN** the view requests the corresponding curve and re-renders

#### Scenario: Empty window degrades gracefully

- **WHEN** the selected window has no best-effort records
- **THEN** the curve view shows an empty-state without erroring
