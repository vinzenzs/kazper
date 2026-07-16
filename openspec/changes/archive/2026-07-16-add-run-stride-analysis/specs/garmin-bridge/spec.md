## MODIFIED Requirements

### Requirement: The bridge extracts and posts the cadence stream defensively

The garmin-bridge SHALL extract a cadence series from the Garmin activity-details payload
alongside the existing power/speed/heart-rate extraction — preferring the bike-cadence
column (`directBikeCadence`, rpm) and falling back to the run-cadence column
(`directDoubleCadence`, Garmin's both-feet series already in steps/min, posted without
halving or doubling) — gaps filled with 0, an all-non-positive series dropped — and include
it as `cadence` (sport-native unit) in the existing per-workout stream POST. A missing or
unexpectedly-shaped cadence descriptor SHALL result in no cadence array and SHALL NOT fail
the extraction, the stream post, or the sync (the `directPower` defensive precedent).

#### Scenario: A ride with cadence data posts four streams

- **WHEN** the activity details carry power, speed, heart-rate, and bike-cadence columns
- **THEN** the stream POST body includes all four arrays

#### Scenario: A run's double cadence is posted as spm

- **WHEN** the activity details carry no bike-cadence column but a `directDoubleCadence`
  column (e.g. a run reporting ~172 steps/min)
- **THEN** the stream POST body includes a `cadence` array holding the steps/min values
  as reported, neither halved nor doubled

#### Scenario: Bike cadence wins when both columns exist

- **WHEN** the activity details carry both `directBikeCadence` and `directDoubleCadence`
  columns
- **THEN** the posted `cadence` array holds the bike-cadence (rpm) series

#### Scenario: A missing cadence descriptor degrades silently

- **WHEN** the activity details carry no recognizable bike- or run-cadence column
- **THEN** the stream POST proceeds without a `cadence` array and the sync completes normally
