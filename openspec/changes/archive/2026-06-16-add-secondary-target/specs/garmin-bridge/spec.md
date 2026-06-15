## MODIFIED Requirements

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, RPE, or absolute HR/power), and SHALL
translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. When a bike step carries a `secondary_target`, the
bridge SHALL additionally emit Garmin's secondary-target fields
(`secondaryTargetType` plus `secondaryZoneNumber` or
`secondaryTargetValueOne`/`secondaryTargetValueTwo`) using the same per-kind
value logic as the primary target. The garminconnect payload shape SHALL exist
only in the bridge and SHALL NOT be returned to or required from the backend.

#### Scenario: A structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: A bike step's secondary target is emitted

- **WHEN** `POST /workouts` is called with a bike step whose primary target is
  `power_zone 4` and whose `secondary_target` is `hr_zone 3`
- **THEN** the executable step carries both the primary power-zone target and the
  Garmin secondary heart-rate-zone fields

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload
