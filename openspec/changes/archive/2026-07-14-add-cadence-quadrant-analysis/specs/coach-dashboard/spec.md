## ADDED Requirements

### Requirement: The workout-detail route renders a quadrant scatter for power-and-cadence rides

The dashboard's `/workouts/:id` route SHALL render a quadrant-analysis scatter (AEPF vs CPV,
reference lines, quadrant shares as a legend) when the workout has stored power and cadence
streams and a fitted critical-power model is available, parameterized from the shared cp-model
fetch with a fixed 90 rpm pivot cadence. When cadence or power is absent, the cp-model is null,
or any fetch fails, the scatter SHALL be absent and the rest of the page unaffected.

#### Scenario: Paired streams with a fitted model show the scatter

- **WHEN** the detail page loads for a ride with power + cadence streams and a fitted cp-model
- **THEN** the quadrant scatter renders with reference lines and the four shares

#### Scenario: Missing cadence hides the scatter

- **WHEN** the workout has no stored cadence stream
- **THEN** the detail page renders without the scatter and without an error
