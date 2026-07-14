## ADDED Requirements

### Requirement: The workout-detail route renders detected intervals for power-streamed rides

The dashboard's `/workouts/:id` route SHALL render a detected-intervals table (ordinal, duration,
average watts, rest after) from `GET /api/v1/workouts/{id}/intervals` when detection returns at
least one interval, positioned alongside the Garmin splits table. When detection reports
`no_distinct_efforts`, the workout has no power stream, or the fetch fails, the table SHALL be
absent and the rest of the page unaffected.

#### Scenario: Detected efforts render as a table

- **WHEN** the detail page loads for a ride where detection finds intervals
- **THEN** the table lists each interval with duration, average power, and the following rest

#### Scenario: A steady ride shows no table

- **WHEN** detection reports `no_distinct_efforts`
- **THEN** no detected-intervals table renders and no error is shown
