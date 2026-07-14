## ADDED Requirements

### Requirement: The workout-detail route renders a W′ balance strip for power-streamed rides

The dashboard's `/workouts/:id` route SHALL render a W′ balance strip when the workout has a
stored power stream and a fitted critical-power model is available: the balance series over time
(downsampled), the minimum marked, and the summary values (minimum, end balance, max depletion),
parameterized from the same cp-model fetch the stats page uses. When the cp-model is null, the
workout has no power stream, or either fetch fails, the strip SHALL be absent — supplementary
detail degrades to absence, not to an error state, and the rest of the detail page renders
unaffected.

#### Scenario: A power-streamed ride with a fitted model shows the strip

- **WHEN** `/workouts/:id` loads for a workout with stored power and the cp-model endpoint
  returns a fitted model
- **THEN** the W′ balance strip renders the series with its minimum marked and the summary values

#### Scenario: Absent prerequisites hide the strip

- **WHEN** the cp-model is null for the window or the workout has no stored power stream
- **THEN** the detail page renders without the strip and without an error
