## ADDED Requirements

### Requirement: The app surfaces Garmin connection and sync status

The companion app SHALL provide a Garmin section (in the settings sheet) that lets the user
(re)connect their Garmin account and see how fresh their Garmin data is. Connecting SHALL
drive the existing login proxy: a connect action calls `POST /garmin/login`; on a
`{"needs_mfa": true}` response the app SHALL prompt for the 6-digit MFA code and call `POST
/garmin/login/mfa` with it; a `{"logged_in": true}` response (from either call) SHALL be
shown as connected. The app SHALL NOT collect or transmit the Garmin email/password — those
remain in the bridge's configuration; the app only triggers login and relays the MFA code.
The bridge's typed login errors (`mfa_invalid`, `bad_credentials`, `locked_out`) SHALL be
surfaced as readable inline messages. The app SHALL display sync status from `GET
/garmin/sync-status` — the last successful sync time, an in-progress indicator when the
latest run is `running`, and a failure indicator when it is `error` — and SHALL treat a
`503 garmin_disabled` response as "Garmin not configured".

#### Scenario: Connect triggers login and prompts for MFA

- **WHEN** the user taps "Connect Garmin" and `POST /garmin/login` returns `{"needs_mfa": true}`
- **THEN** the app shows a 6-digit code field
- **AND** submitting the code calls `POST /garmin/login/mfa` and shows the connected state on success

#### Scenario: Login without MFA connects directly

- **WHEN** `POST /garmin/login` returns `{"logged_in": true}`
- **THEN** the app shows the connected state without prompting for a code

#### Scenario: An MFA error is shown inline

- **WHEN** `POST /garmin/login/mfa` returns an `mfa_invalid` error
- **THEN** the app shows an inline "code was wrong or expired" message and lets the user re-enter the code

#### Scenario: Sync status shows last successful sync

- **WHEN** the Garmin section loads and `GET /garmin/sync-status` returns a `last_successful_at`
- **THEN** the app shows the relative time since that sync (e.g. "Last synced 2 h ago")

#### Scenario: A failed or in-progress sync is distinguished

- **WHEN** the latest run is `status=error` (or `running`)
- **THEN** the app shows a failure indicator (or a "syncing…" indicator) rather than implying the data is fresh

#### Scenario: Garmin unconfigured degrades gracefully

- **WHEN** the Garmin endpoints return `503 garmin_disabled`
- **THEN** the app shows "Garmin not configured" rather than an error
