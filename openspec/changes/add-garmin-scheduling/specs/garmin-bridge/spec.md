# garmin-bridge — delta for add-garmin-scheduling

## ADDED Requirements

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, RPE, or absolute HR/power), and SHALL
translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. The garminconnect payload shape SHALL exist only in
the bridge and SHALL NOT be returned to or required from the backend.

#### Scenario: A structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload

### Requirement: The bridge schedules and unschedules workouts on the calendar

The bridge SHALL expose `POST /schedule` accepting a Garmin workout id and a
date, placing that workout on the Garmin calendar and returning the Garmin
schedule id; and `DELETE /schedule` accepting a Garmin schedule id, removing the
scheduled entry. Deleting an already-absent schedule id SHALL succeed as a no-op.

#### Scenario: Scheduling returns a schedule id

- **WHEN** `POST /schedule` is called with a Garmin workout id and a date
- **THEN** the workout is placed on that date and the response carries the Garmin schedule id

#### Scenario: Unscheduling is idempotent

- **WHEN** `DELETE /schedule` is called with a schedule id that is already gone
- **THEN** the response indicates success (no-op)

### Requirement: The bridge reads the Garmin calendar for a date range

The bridge SHALL expose `GET /calendar` accepting a date range and returning the
scheduled workouts in that range, for reconciliation by the backend.

#### Scenario: Calendar read returns scheduled items

- **WHEN** `GET /calendar` is called with a from/to range that contains scheduled workouts
- **THEN** the response lists those scheduled items with their Garmin schedule ids
