## ADDED Requirements

### Requirement: The companion registers for push and handles the Garmin relogin notification

The mobile companion SHALL request notification permission and register its FCM
registration token with the backend (`POST /push/tokens`) on pairing and on every
token refresh, and SHALL drop the token (`DELETE /push/tokens`) on unpair/sign-out.
On receiving a push whose data payload carries the relogin action
(`action:"garmin_relogin"`), the app SHALL route the user into the existing Garmin
re-authentication flow (`POST /garmin/login` → `POST /garmin/login/mfa`) rather than a
generic screen, both from a cold tap on the system notification and from an in-app
foreground message. Push is a supplement: the app SHALL still surface staleness from
`GET /garmin/sync-status` so a missed or undelivered push is not the only signal.

#### Scenario: Token is registered on pairing

- **WHEN** the app is paired and notification permission is granted
- **THEN** it obtains its FCM token and `POST`s it to `/push/tokens`
- **AND** re-registers whenever FCM rotates the token

#### Scenario: Token is dropped on sign-out

- **WHEN** the user unpairs or signs out
- **THEN** the app `DELETE`s its token at `/push/tokens`

#### Scenario: Relogin notification deep-links to re-authentication

- **WHEN** the app receives a push with `action:"garmin_relogin"` (tapped from the tray or received in foreground)
- **THEN** the app opens the Garmin re-authentication flow driving `POST /garmin/login` and `POST /garmin/login/mfa`

#### Scenario: Sync staleness remains visible without a push

- **WHEN** no relogin push was delivered but `GET /garmin/sync-status` reports `is_stale`
- **THEN** the app still surfaces the stale-sync state to the user
