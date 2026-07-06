## ADDED Requirements

### Requirement: The stats route surfaces volume totals and an activity heatmap

The dashboard SHALL provide a `/stats` route that reads `GET /api/v1/workouts/summary` and
presents training volume as a "training log" view: a period toggle (Week · Month · Year-to-date)
that selects the range, totals cards for the selected window (total distance, total moving/elapsed
time, total elevation gain, activity count, and a by-sport breakdown), and a calendar activity
heatmap built from the endpoint's per-day series. Distance and elevation SHALL be rendered in
human units (e.g. km) from the metres on the wire. The view SHALL reuse the existing analyst
visual language (`Panel` / `Stat` and the visx idiom) with no celebratory/social treatment, and
SHALL degrade to an empty-state when the window has no completed workouts rather than erroring.
The header nav SHALL gain a Stats link.

#### Scenario: Volume totals render for the selected period

- **WHEN** the user opens `/stats` and `GET /api/v1/workouts/summary` returns completed workouts
  for the selected period
- **THEN** the view shows totals cards (distance, time, elevation, count, by-sport) and the
  activity heatmap for that window

#### Scenario: Period toggle changes the window

- **WHEN** the user switches the toggle between Week, Month, and Year-to-date
- **THEN** the view requests the corresponding range and re-renders the totals and heatmap

#### Scenario: Empty window degrades gracefully

- **WHEN** the selected period has no completed workouts
- **THEN** the stats route shows an empty-state without erroring
