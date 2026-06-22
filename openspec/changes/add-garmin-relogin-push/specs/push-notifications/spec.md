## ADDED Requirements

### Requirement: Device push tokens are registered in a dedicated table

The system SHALL persist Android push (FCM) registration tokens in a dedicated `push_tokens` table, one row per device token, holding the opaque FCM registration `token`, a `platform` (today always `android`), and audit timestamps. Registration SHALL be an upsert keyed by `token` so the mobile companion can re-register the same token idempotently (FCM rotates tokens; a rotated token is a new row, a refreshed identical token is a no-op upsert). Tokens are device identifiers, not nutrition data, and SHALL NOT feed any computation.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `push_tokens` exists with `id` (UUID PK), `token` (TEXT NOT NULL), `platform` (TEXT NOT NULL DEFAULT `android`), `created_at`, `updated_at`
- **AND** a UNIQUE index exists on `(token)` so re-registration upserts in place

### Requirement: The mobile identity registers and removes push tokens

The system SHALL expose `POST /push/tokens` that upserts a device token (`{"token":"<fcm-token>"}`, optional `platform`) and `DELETE /push/tokens` that removes a supplied token (`{"token":"<fcm-token>"}`). Both endpoints SHALL be restricted to the `mobile` identity — the `agent` and `garmin` identities receive `403 forbidden`. These endpoints are device-specific and SHALL NOT be mirrored as MCP tools. When push is unconfigured the endpoints SHALL still accept and store tokens (registration is independent of the sender being configured), so a later configuration change can deliver without re-pairing.

#### Scenario: Mobile registers a token

- **WHEN** the `mobile` identity `POST`s `/push/tokens` with `{"token":"abc123"}`
- **THEN** the response is `201` (or `200` on re-register) and a `push_tokens` row exists for `abc123`

#### Scenario: Re-registering the same token is idempotent

- **WHEN** the same token is registered twice
- **THEN** exactly one row exists for it and `updated_at` is refreshed

#### Scenario: Removing a token

- **WHEN** the `mobile` identity `DELETE`s `/push/tokens` with `{"token":"abc123"}`
- **THEN** the row is removed and a subsequent send skips it

#### Scenario: Non-mobile identities are forbidden

- **WHEN** the `agent` or `garmin` identity calls `POST /push/tokens` or `DELETE /push/tokens`
- **THEN** the response is `403 forbidden`

### Requirement: The FCM sender is opt-in and gated on configuration

The system SHALL send Android push via the FCM HTTP v1 API, authenticating with a configured Google service-account credential (`FCM_SERVICE_ACCOUNT_JSON`, `FCM_PROJECT_ID`). The sender SHALL mint a short-lived OAuth2 access token from the service-account JWT and call `messages:send` per device token. When push is unconfigured (either key unset) the whole push surface is disabled: send is a silent no-op and `503 push_disabled` is returned by any push operation that requires the sender. The service-account JSON MUST NOT appear in any log line and SHALL be redacted from config dumps.

#### Scenario: Push disabled when unconfigured

- **WHEN** `FCM_PROJECT_ID` or `FCM_SERVICE_ACCOUNT_JSON` is unset and a relogin condition occurs
- **THEN** no FCM call is made and no error is surfaced to the triggering request

#### Scenario: Send fans out to every registered token

- **WHEN** the sender is configured and two tokens are registered
- **THEN** a `messages:send` call is issued for each token

#### Scenario: A token FCM rejects as unregistered is pruned

- **WHEN** FCM returns `UNREGISTERED` / `NOT_FOUND` (`404`) for a token during send
- **THEN** that `push_tokens` row is deleted so it is not retried on the next send
- **AND** the remaining tokens are still attempted

#### Scenario: The credential never appears in logs

- **WHEN** any startup, config-dump, or error log line references push configuration
- **THEN** no substring of the raw service-account JSON appears (presence/absence only)

### Requirement: A Garmin relogin-needed push is sent once per outage

The system SHALL send a relogin-needed push notification when a Garmin sync run is closed with `status=error` AND the stored Garmin token is absent (the bridge cleared it on auth failure). The notification SHALL carry a human-readable title/body prompting re-authentication and a data payload identifying the relogin action so the app can deep-link. A single-row latch SHALL guard against repeat notifications: once a relogin push is sent the latch is set, and further error-closes with an absent token while the latch is set SHALL NOT re-send. The latch SHALL be cleared when a fresh token is stored (`PUT /garmin/token`) or a sync run is closed `status=success`, so the next distinct outage notifies again.

#### Scenario: First failing sync with a cleared token notifies

- **WHEN** a sync run is closed `status=error` and `GET /garmin/token` would return `404`, and the latch is unset
- **THEN** a relogin push is sent to every registered token AND the latch is set

#### Scenario: A failing sync while the token still exists does not notify

- **WHEN** a sync run is closed `status=error` but the Garmin token is still stored (a transient error such as a rate-limit)
- **THEN** no relogin push is sent and the latch is unchanged

#### Scenario: Repeated failures during the same outage do not re-notify

- **WHEN** the latch is already set and another error-close with an absent token occurs
- **THEN** no additional push is sent

#### Scenario: Recovery clears the latch

- **WHEN** a fresh token is stored via `PUT /garmin/token`, or a sync run closes `status=success`
- **THEN** the latch is cleared so a future outage will notify again

#### Scenario: Relogin push carries a deep-link action

- **WHEN** a relogin push is sent
- **THEN** its data payload includes an action identifier (e.g. `action:"garmin_relogin"`) the companion uses to route to the re-authentication flow
