## Why

`add-garmin-relogin-push` shipped the backend half of push: when the Garmin token
expires and a sync fails, the server sends an FCM relogin notification to every registered
device token. But the companion app does the other half of *nothing* — it has no Firebase
dependency, registers no token, and can't receive or act on a push. So today the feature is
inert end-to-end: the server would push into the void. This change wires the Android
companion so a relogin push actually reaches the phone and deep-links into the Garmin
connect flow that already exists — closing the loop the backend was built for.

## What Changes

- **Firebase Messaging in the app (Android).** Add `firebase_core` + `firebase_messaging`,
  initialize Firebase at startup, and request the Android 13+ notification permission.
- **Token registration tied to pairing.** When the app is paired, obtain the FCM
  registration token and `POST /push/tokens`; re-register on `onTokenRefresh`; `DELETE
  /push/tokens` on unpair. (These endpoints already exist, mobile-identity only.)
- **Receive + act on the relogin push.** A background message handler plus foreground
  handling: a notification carrying the `garmin_relogin` data action deep-links into the
  existing Garmin connect sheet (`showGarminSheet`) and refreshes sync status. System tray
  display is handled by FCM when backgrounded; foreground shows a lightweight in-app prompt.
- **Android native plumbing.** Apply the `com.google.gms.google-services` Gradle plugin
  (Kotlin DSL) and document the **manual** `google-services.json` prerequisite (downloaded
  from the Firebase console; gitignored, not committed).
- **Docs.** A short "enabling push" section (Firebase project + the two server env vars +
  the app's `google-services.json`) — the doc update the backend change deferred.

## Capabilities

### New Capabilities
<!-- none — the backend push-notifications capability already exists -->

### Modified Capabilities
- `mobile-companion`: the app SHALL register for and act on Garmin relogin push
  notifications — register its FCM token while paired, and deep-link a relogin push into
  the Garmin connect flow.

## Impact

- **Companion app** (`apps/companion/`): `pubspec.yaml` (+2 deps); `android/` Gradle +
  manifest (google-services plugin, `POST_NOTIFICATIONS`); `main.dart` (Firebase init +
  background handler `@pragma('vm:entry-point')`); a new `data/push/` (FCM token source) +
  `state/push_provider.dart` (registration tied to pairing) + message routing into
  `showGarminSheet`.
- **Manual prerequisite (not code):** a Firebase project + an Android app registration, and
  `apps/companion/android/app/google-services.json` placed by the operator. Documented, not
  committed.
- **No backend change** — `POST`/`DELETE /push/tokens` and the FCM sender already exist; the
  server gates push on `FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON`.
- **Scope: Android only.** The companion is an Android app (APK build; backend `platform`
  defaults to `android`). iOS/APNs is out of scope.
- **Docs:** `RUN_LOCAL.md` (+ `.env.local` example) gain the push opt-in section.
