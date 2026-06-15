## MODIFIED Requirements

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, cadence, RPE, or absolute HR/power),
and SHALL translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. A `cadence` target SHALL be emitted as Garmin's
cadence target type with `targetValueOne=low` and `targetValueTwo=high`. The
garminconnect payload shape SHALL exist only in the bridge and SHALL NOT be
returned to or required from the backend.

#### Scenario: A structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: A cadence target is emitted as a Garmin cadence gate

- **WHEN** `POST /workouts` is called with a run step targeting
  `{kind:"cadence", low:88, high:92}`
- **THEN** the bridge emits a Garmin cadence target with `targetValueOne=88`,
  `targetValueTwo=92`

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload
