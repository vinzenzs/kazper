## ADDED Requirements

### Requirement: The app registers for and acts on Garmin relogin push

The companion app SHALL integrate Firebase Cloud Messaging (Android) so the backend's Garmin
relogin push reaches the device and routes the user to re-authentication. While the app is
paired, it SHALL obtain its FCM registration token and register it via `POST /push/tokens`
(`{token, platform:"android"}`, the `mobile` identity), SHALL re-register when the token
refreshes, and SHALL deregister via `DELETE /push/tokens` on unpair. Token registration SHALL
proceed even when the OS notification permission is denied (so enabling notifications later
needs no re-pair). On receiving a push whose data `action` is `garmin_relogin`, the app
SHALL route the user into the existing Garmin connect flow (the Garmin connect sheet) and
refresh sync status; a tap on the tray notification (from background or cold start) SHALL
open that flow, and a push received in the foreground SHALL surface a lightweight in-app
prompt rather than a silent drop. Pushes with any other or absent `action` SHALL be ignored
without error. The Firebase project credential (`google-services.json`) is an operator-
supplied, uncommitted prerequisite; this is documented, not bundled.

#### Scenario: A paired device registers its token

- **WHEN** the app is paired and Firebase yields an FCM token
- **THEN** the app `POST`s `/push/tokens` with `{token, platform:"android"}` under the mobile identity

#### Scenario: Token refresh re-registers

- **WHEN** Firebase rotates the registration token
- **THEN** the app `POST`s the new token to `/push/tokens`

#### Scenario: Unpair deregisters

- **WHEN** the user unpairs
- **THEN** the app `DELETE`s `/push/tokens` for its token

#### Scenario: Registration survives a denied permission

- **WHEN** the OS notification permission is denied but the app is paired
- **THEN** the token is still registered with the backend
- **AND** the Garmin settings section shows a "notifications off" hint

#### Scenario: A relogin push opens the Garmin connect flow

- **WHEN** a push with data `action = "garmin_relogin"` is tapped (from background or cold start)
- **THEN** the app opens the Garmin connect sheet
- **AND** refreshes the Garmin sync status

#### Scenario: A foreground relogin push prompts in-app

- **WHEN** the same push arrives while the app is foregrounded
- **THEN** the app shows an in-app prompt with a reconnect action (not a silent drop)

#### Scenario: An unknown action is ignored safely

- **WHEN** a push arrives with no `action` or an unrecognized one
- **THEN** the app ignores it without error or navigation
