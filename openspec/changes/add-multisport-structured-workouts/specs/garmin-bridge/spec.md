## MODIFIED Requirements

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, RPE, or absolute HR/power), and SHALL
translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. The endpoint SHALL additionally accept a **multisport**
form — an ordered list of segments, each with its own `sport` and step program
(plus `transition` segments) — and compile it into a payload with multiple
`workoutSegments` entries: one per segment, each with its own `sportType`, a
monotonic `segmentOrder`, and `workoutSteps` numbered by a step-order counter
that spans the whole workout; `transition` segments map to Garmin's transition
sport type. The garminconnect payload shape SHALL exist only in the bridge and
SHALL NOT be returned to or required from the backend.

#### Scenario: A single-sport structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: A multisport workout compiles to multiple segments

- **WHEN** `POST /workouts` is called with a multisport form of segments
  `[swim, transition, bike, transition, run]`
- **THEN** the payload has five ordered `workoutSegments`, each with its own
  `sportType` (transition segments using the transition sport type)
- **AND** step order is monotonic across all segments
- **AND** one Garmin workout id is returned

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport/segments, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload
