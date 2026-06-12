# garmin-bridge Specification

## Purpose

Provide a small, stateless bridge between Garmin Connect and the nutrition REST
API. It handles the interactive multi-factor login needed to mint a Garmin auth
token, persists that token to the backend (it holds no durable local state
itself), and runs a headless daily sync that fetches a day's Garmin data and
maps it onto the existing REST endpoints under the garmin identity.

## Requirements

### Requirement: Interactive MFA login mints and persists a Garmin token

The bridge SHALL expose a two-step login that performs Garmin SSO with
credentials it reads from its own configuration (never from the request body),
and SHALL handle multi-factor auth: `POST /login` begins the flow and, when MFA
is required, responds indicating a code is needed; `POST /login/mfa` completes
the flow with the supplied code. On success the bridge SHALL persist the minted
token blob to the backend via `PUT /garmin/token` and SHALL NOT return the blob
to the caller. The Garmin password SHALL NOT appear in any response or log.

#### Scenario: Login requiring MFA

- **WHEN** `POST /login` is called and Garmin requires MFA
- **THEN** the response indicates an MFA code is needed (e.g. `{"needs_mfa": true}`)
- **AND** the bridge retains the in-progress SSO state to resume with the code

#### Scenario: Completing MFA persists the token

- **WHEN** `POST /login/mfa` is called with the correct 6-digit code
- **THEN** login completes and the minted token blob is sent to the backend via `PUT /garmin/token`
- **AND** the response confirms success without returning the token

#### Scenario: Credentials never transit the request or logs

- **WHEN** any login request is processed
- **THEN** the Garmin password is taken from configuration, not the request
- **AND** neither the password nor the token blob appears in logs or responses

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; weigh-ins → `/weight`; activities → `/workouts`
(`source = "garmin"`). Sync SHALL require no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, weight, and
  activity data to their respective endpoints under the garmin identity

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing

### Requirement: The bridge is stateless except during the login window

The bridge SHALL hold no durable local state: the auth token lives in the
backend, so a restarted or rescheduled bridge resumes by reading it. The only
in-memory state is the transient SSO context between `POST /login` and
`POST /login/mfa`; because of it the bridge SHALL run as a single replica.

#### Scenario: Restart between syncs loses nothing

- **WHEN** the bridge process restarts between two daily syncs
- **THEN** the next `POST /sync` reads the token from the backend and proceeds
- **AND** no re-login is required

#### Scenario: Single replica for the login handshake

- **WHEN** the bridge is deployed
- **THEN** it runs with a single replica so the MFA resume reaches the pod that
  began the login

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
