## 1. Dependencies + Android native

- [x] 1.1 `pubspec.yaml`: add `firebase_core` and `firebase_messaging` (pinned to current stable).
- [x] 1.2 Android Gradle (Kotlin DSL): apply the `com.google.gms.google-services` plugin in `android/settings.gradle.kts` (plugin declaration) + `android/app/build.gradle.kts` (apply); ensure minSdk meets Firebase's floor.
- [x] 1.3 `AndroidManifest.xml`: add the `POST_NOTIFICATIONS` permission (Android 13+); optional default notification channel/icon metadata.
- [x] 1.4 Gitignore + document `apps/companion/android/app/google-services.json` as the operator-supplied prerequisite (build fails loudly if missing — note this in the doc). (gitignore already in root `.gitignore`; doc note in 6.1)

## 2. Firebase init + background handler

- [x] 2.1 `main.dart`: `await Firebase.initializeApp()` before `runApp`.
- [x] 2.2 Top-level `@pragma('vm:entry-point')` `firebaseMessagingBackgroundHandler` (mirroring `callbackDispatcher`): minimal, never reads the provider graph; registered via `FirebaseMessaging.onBackgroundMessage`.

## 3. Push data layer

- [x] 3.1 `data/push/push_messaging.dart`: a small interface over `firebase_messaging` (getToken, onTokenRefresh stream, onMessage / onMessageOpenedApp / getInitialMessage, requestPermission) + the real impl, so the provider is fakeable in tests.
- [x] 3.2 Provider wiring for the messaging port in `app_providers.dart` (overridable in tests).

## 4. Registration tied to pairing

- [x] 4.1 `state/push_provider.dart`: on paired → request permission, `getToken()`, `POST /push/tokens` (`api.dio` direct, like the Garmin provider); on `onTokenRefresh` → re-POST; on unpair → `DELETE /push/tokens`.
- [x] 4.2 Register the token even when permission is denied; expose a "notifications enabled?" signal for the settings hint.
- [x] 4.3 Hook the provider into the app lifecycle (kick it from the paired branch in `main.dart`/`home_shell`, alongside the existing pairing gate).

## 5. Message routing → Garmin connect

- [x] 5.1 Route `data.action == "garmin_relogin"` (from `onMessageOpenedApp` + `getInitialMessage`) → `showGarminSheet(context)` on the home navigator + `ref.invalidate(garminSyncProvider)`.
- [x] 5.2 Foreground (`onMessage`): show a SnackBar/banner with a "Reconnect" action that opens the sheet.
- [x] 5.3 Ignore unknown/absent actions without error.
- [x] 5.4 Add a "notifications off" hint to the Garmin section in `settings_sheet.dart` when permission is denied.

## 6. Docs

- [x] 6.1 `RUN_LOCAL.md`: an "Enabling push" section — create a Firebase project + Android app, download `google-services.json` to `android/app/`, set the server's `FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON` (cross-link the backend half), and the two-sided enable note.
- [x] 6.2 `.env.local` example (and README if it lists env vars): add the `FCM_*` opt-in keys (the doc update `add-garmin-relogin-push` deferred). (`.env.example` already carried them from the backend change; README config table now lists them.)

## 7. Tests + verification

- [x] 7.1 Push-provider tests with a fake messaging port: registers on pair, re-registers on token refresh, deregisters on unpair, registers despite denied permission.
- [x] 7.2 Routing test: a `garmin_relogin` message produces the open-sheet + refresh intent; an unknown action is a no-op.
- [x] 7.3 `flutter analyze` clean; `flutter test` green (full companion suite — 89 tests).
- [x] 7.4 Manual smoke (documented, not automated): with server `FCM_*` set + a real `google-services.json`, force a Garmin error-close with the token absent and confirm the device receives the push and the tap opens the Garmin sheet. (Procedure documented in RUN_LOCAL.md "Enabling push"; requires a real device + Firebase project to execute.)
