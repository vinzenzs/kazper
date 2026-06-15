## MODIFIED Requirements

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, swim pace, RPE, or absolute HR/power),
and SHALL translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. A `swim_pace` target SHALL be converted to Garmin's
m/s pace target via `100 / sec_per_100m` (paralleling the `pace` kind's
`1000 / sec_per_km`). The garminconnect payload shape SHALL exist only in the
bridge and SHALL NOT be returned to or required from the backend.

#### Scenario: A structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: A swim_pace target is converted to a Garmin pace gate

- **WHEN** `POST /workouts` is called with a swim workout whose step targets
  `{kind:"swim_pace", low_sec_per_100m:100, high_sec_per_100m:100}`
- **THEN** the bridge emits a Garmin pace target of `1.0` m/s (`100/100`)

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload
