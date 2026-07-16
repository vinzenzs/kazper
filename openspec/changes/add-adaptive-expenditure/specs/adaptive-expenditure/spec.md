## ADDED Requirements

### Requirement: Energy expenditure is estimated from intake and the weight trend

The system SHALL expose `GET /api/v1/nutrition/expenditure?from=&to=&tz=` estimating average
daily energy expenditure over the window by energy balance:
`mean(intake over logged days) − (trend_end − trend_start) × 7700 kcal/kg ÷ window_days`, where
the mass signal is the body-weight capability's existing smoothed trend evaluated at the window
ends (values and their dates echoed in the response). A day SHALL count as logged only when it
has at least one meal; unlogged days SHALL be excluded from the intake mean and reported as
`days_unlogged`, never treated as zero intake. The response SHALL carry
`expenditure_kcal_per_day` (1 decimal), the window, the trend endpoints with `delta_kg`, and
`{mean_kcal_logged_days, days_logged, days_unlogged}`. Fewer than **14 logged days** SHALL
degrade to `200` with a null estimate and `reason: "insufficient_logged_days"`; fewer than
**5 weigh-ins** in the window SHALL degrade with `reason: "insufficient_weigh_ins"`. The range
SHALL be capped at 92 days with the shared range vocabulary. The endpoint SHALL be
compute-on-read, persist nothing, and read neither goals nor athlete-config — the estimate is
advisory and goal comparison belongs to the consumer.

#### Scenario: A well-logged window returns the balance estimate

- **WHEN** the window holds 25 logged days averaging 2,800 kcal and the weight trend fell
  0.5 kg across it
- **THEN** the response reports `expenditure_kcal_per_day ≈ 2800 + (0.5 × 7700 / days)` with
  the trend endpoints, delta, and day counts

#### Scenario: Unlogged days are excluded and counted

- **WHEN** 6 days in the window have no meals
- **THEN** those days are absent from the intake mean and `days_unlogged` reports 6

#### Scenario: Sparse logging degrades honestly

- **WHEN** only 9 days in the window have meals
- **THEN** the response is `200` with a null estimate and `reason: "insufficient_logged_days"`

#### Scenario: Too few weigh-ins degrade honestly

- **WHEN** the window contains 3 weigh-ins
- **THEN** the response is `200` with a null estimate and `reason: "insufficient_weigh_ins"`

### Requirement: The expenditure estimate is readable over MCP

The system SHALL expose an `energy_expenditure` MCP tool (read tier) issuing a single
`GET /api/v1/nutrition/expenditure` and forwarding the body verbatim (`from`/`to` required,
`tz` optional). The description SHALL recommend 21–28 day windows, warn against reading across
deliberate glycogen manipulation (race week), note the under-logging bias, and point at the
existing goals endpoints for comparing and applying targets.

#### Scenario: Agent reads the estimate in one call

- **WHEN** the agent invokes `energy_expenditure` over the trailing 28 days
- **THEN** the tool issues one GET and returns the estimate and its inputs verbatim
