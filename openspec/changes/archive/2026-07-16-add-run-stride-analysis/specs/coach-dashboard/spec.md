## ADDED Requirements

### Requirement: The workout-detail route renders a cadence-vs-stride view for runs

The dashboard's `/workouts/:id` route SHALL render a cadence-vs-stride view for run
workouts with stored speed and cadence streams, fetched from `GET /workouts/{id}/stride`:
the speed-binned mean cadence and mean step length (the plateau view) plus the contribution
split when present; when the split is `null` the view SHALL show the insufficient-range
reason rather than an empty chart. When the workout is not a run, either stream is absent,
or the fetch fails, the view SHALL be absent and the rest of the page unaffected.

#### Scenario: A run with paired streams shows the stride view

- **WHEN** the detail page loads for a run with speed + cadence streams and a computable
  contribution split
- **THEN** the cadence-vs-stride view renders the binned series and the split

#### Scenario: A steady run explains itself

- **WHEN** the stride response carries a `null` split with
  `reason: "insufficient_speed_range"`
- **THEN** the view renders the bins with the reason text instead of a split

#### Scenario: Missing cadence hides the view

- **WHEN** the run has no stored cadence stream
- **THEN** the detail page renders without the stride view and without an error
