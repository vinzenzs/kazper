## ADDED Requirements

### Requirement: Windowed best-effort aggregates are sport-scoped

Every windowed aggregation over best-effort records — the power/pace curve, the CP model and its
history, the power profile, and durability — SHALL attribute efforts only from workouts of the
sport the read answers for: the `power` metric SHALL aggregate exclusively from `bike` workouts,
and the `speed` metric exclusively from the single requested `run` or `swim` sport. A workout of
another sport that stored rows for the same metric (e.g. a run with a running-power series, or a
bike's speed series) SHALL never contribute to that window. Multisport workouts SHALL match no
sport-scoped window. Per-workout stream computations (W′ balance, quadrant, intervals, execution
metrics) are exempt — they answer for the workout itself.

#### Scenario: Running power never enters cycling analytics

- **WHEN** a completed run stored power best-efforts (running power) beside completed bike
  workouts in the window
- **THEN** the power curve, CP fit points, power-profile anchors, and durability tiers derive
  exclusively from the bike workouts

#### Scenario: Bike speed never enters the run pace curve

- **WHEN** a bike workout stored speed best-efforts in the window
- **THEN** `GET /workouts/power-curve?sport=run` returns only run workouts' speed efforts

#### Scenario: A window emptied by scoping degrades honestly

- **WHEN** the only power rows in a window came from non-bike workouts
- **THEN** the curve is empty and the CP model gates with its existing reasons, rather than
  fitting foreign data

### Requirement: The CP fit carries a fit-quality warning

The CP model response (and each fitted history anchor) SHALL carry `warning: "poor_fit"` when
`r_squared < 0.5`, returning the fitted values regardless — quality is an advisory axis
independent of the point-count and span gates. The `cp_model`/`cp_model_history` MCP tool
descriptions SHALL note the warning's meaning.

#### Scenario: A weak fit is returned but flagged

- **WHEN** the in-band points fit with `r_squared = 0.31`
- **THEN** the model returns with `warning: "poor_fit"`

#### Scenario: A sound fit carries no warning

- **WHEN** the fit's `r_squared` is 0.93
- **THEN** no `warning` field is present
