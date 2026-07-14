## ADDED Requirements

### Requirement: The bridge extracts and posts the cadence stream defensively

The garmin-bridge SHALL extract a bike-cadence series (`directBikeCadence`) from the Garmin
activity-details payload alongside the existing power/speed/heart-rate extraction — gaps filled
with 0, an all-non-positive series dropped — and include it as `cadence` (rpm) in the existing
per-workout stream POST. A missing or unexpectedly-shaped cadence descriptor SHALL result in no
cadence array and SHALL NOT fail the extraction, the stream post, or the sync (the
`directPower` defensive precedent).

#### Scenario: A ride with cadence data posts four streams

- **WHEN** the activity details carry power, speed, heart-rate, and bike-cadence columns
- **THEN** the stream POST body includes all four arrays

#### Scenario: A missing cadence descriptor degrades silently

- **WHEN** the activity details carry no recognizable bike-cadence column
- **THEN** the stream POST proceeds without a `cadence` array and the sync completes normally
