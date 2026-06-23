## Context

The backend (`add-garmin-relogin-push`) already: stores FCM registration tokens
(`push_tokens`, migration 052) via `POST`/`DELETE /push/tokens` (mobile identity only);
sends via FCM HTTP v1 (`internal/push/fcm.go`) when `FCM_PROJECT_ID` +
`FCM_SERVICE_ACCOUNT_JSON` are set; and fires a relogin push as a side-effect of an
error-close sync run with the Garmin token absent. The push payload carries a tray
`notification` (title/body) plus a `data` action (`garmin_relogin`).

The companion (`apps/companion/`) is an Android Flutter app: Riverpod state, a `dio`
`ApiClient` whose interceptor injects the `mobile` bearer token, `flutter_secure_storage`,
a pairing lifecycle (`pairingProvider` → `PairPage` vs `HomeShell`), a background isolate
pattern in `main.dart` (`callbackDispatcher`, `@pragma('vm:entry-point')`), Kotlin-DSL
Gradle, and — already shipped — a Garmin connect sheet (`showGarminSheet`) with the
login/MFA flow. None of Firebase exists yet.

This change is the app half: receive the push and route it into the sheet that already
exists. It is deliberately thin — no new backend, no new screens.

## Goals / Non-Goals

**Goals:**
- A real device registers its FCM token while paired and drops it on unpair.
- A relogin push, tapped, opens the Garmin connect sheet; sync-status refreshes.
- The Firebase project + `google-services.json` setup is documented, not guessed.

**Non-Goals:**
- **iOS / APNs.** The companion is Android-only; out of scope.
- **A general notification framework.** Only the `garmin_relogin` action is handled; other
  actions are ignored (forward-compatible, no crash).
- **Committing `google-services.json`.** It is operator-supplied and gitignored.
- **Changing the backend** — token endpoints + sender already exist.
- In-app notification preferences / channels beyond the one relogin channel.

## Decisions

### 1. Registration is tied to the pairing lifecycle

The app already gates `PairPage` vs `HomeShell` on `pairingProvider`. Token registration
follows the same signal: when paired (and after notification permission is resolved),
`getToken()` → `POST /push/tokens {token, platform:"android"}`; on `onTokenRefresh`,
re-`POST`; on unpair, `DELETE /push/tokens` with the last token. Registration is
idempotent server-side (upsert by token), so re-registering on every launch is safe and is
the recovery path if a prior POST failed offline.

**Alternative considered:** register once at first launch. Rejected — token rotation and
reinstalls would silently break delivery; cheap re-register on launch is robust.

### 2. Message routing → the existing Garmin sheet

Three entry points, one destination:
- **Background/terminated tap:** `FirebaseMessaging.onMessageOpenedApp` (and
  `getInitialMessage` for cold start) → if `data.action == "garmin_relogin"`, open
  `showGarminSheet` on the home navigator and invalidate `garminSyncProvider`.
- **Foreground:** `FirebaseMessaging.onMessage` → show a lightweight in-app prompt
  (SnackBar/banner with a "Reconnect" action) rather than hijacking the screen, since
  Android suppresses the tray notification while foregrounded.
- **Background data handling:** a top-level `@pragma('vm:entry-point')`
  `firebaseMessagingBackgroundHandler` (mirroring `callbackDispatcher`) — minimal, since the
  `notification` payload makes Android render the tray entry itself; the handler exists for
  forward-compatible data-only messages and must not touch the provider graph.

Routing keys off the `data.action` string the backend already sends, so app and server stay
decoupled from the notification's display copy.

### 3. Permission + the Android 13+ gate

`firebase_messaging`'s `requestPermission()` covers the `POST_NOTIFICATIONS` runtime
permission (Android 13+); `permission_handler` is already a dependency for the settings
deep-link if the user denies. Requesting happens after pairing (we don't prompt an unpaired
user). A denied permission still registers the token (so enabling notifications later in OS
settings works without re-pairing) but surfaces a one-line "notifications off" hint in the
Garmin settings section.

### 4. `google-services.json` is a documented manual step

The file is project-specific config (Android app id, API key, sender id). It is **not**
committed (gitignored) — the operator downloads it from the Firebase console and drops it at
`apps/companion/android/app/google-services.json`. The Gradle `google-services` plugin
(Kotlin DSL) reads it at build time. The build SHALL fail clearly if it's missing rather
than silently shipping a no-FCM APK — document this so the failure is understood.

### 5. A thin `push` data layer, not a god-provider

`data/push/push_messaging.dart` wraps `firebase_messaging` behind a small interface (get
token, token-refresh stream, message streams) so `state/push_provider.dart` — which owns the
register/deregister-on-pairing logic — is unit-testable with a fake, matching how the app
fakes `Repository`/`ApiClient` elsewhere. The provider calls `api.dio` directly for
`/push/tokens` (consistent with the Garmin provider's direct-dio precedent).

## Risks / Trade-offs

- **[No `google-services.json` → build breaks]** → Intended and documented; better a loud
  build failure than a silent no-push APK. Dev without push: skip adding the plugin locally,
  or use a placeholder Firebase project.
- **[Push won't deliver until server FCM keys are also set]** → Independent halves; the app
  registers tokens regardless, so turning on server keys later "just works" with no app
  change. Documented as a two-sided enable.
- **[Background isolate footguns]** → The background handler stays minimal and never reads
  Riverpod (same discipline as the existing `callbackDispatcher`).
- **[Firebase init cost at startup]** → `Firebase.initializeApp()` is one await in `main()`;
  negligible, and the app already awaits Prefs + WorkManager there.

## Migration Plan

No DB migration (app-only). Sequence:
1. `pubspec.yaml`: `firebase_core`, `firebase_messaging`.
2. Android: apply `com.google.gms.google-services` (Kotlin DSL, root + app Gradle),
   `POST_NOTIFICATIONS` in the manifest; document the manual `google-services.json`.
3. `main.dart`: `Firebase.initializeApp()`; register the background handler.
4. `data/push/` interface + `state/push_provider.dart` (register/deregister on pairing,
   token-refresh, permission).
5. Message routing into `showGarminSheet` + `garminSyncProvider` invalidation; foreground
   prompt; a "notifications off" hint in the Garmin settings section.
6. `RUN_LOCAL.md` + `.env.local` push section.
7. Tests: push provider (register-on-pair, deregister-on-unpair, refresh) with a fake
   messaging port; routing maps `garmin_relogin` → open-sheet intent.

## Open Questions

- **Foreground UX** — SnackBar vs a small banner. Lean SnackBar with a "Reconnect" action;
  confirm at apply.
- **flutterfire_cli** — use it to generate `firebase_options.dart`, or rely solely on
  `google-services.json`? Lean on `google-services.json` only (one less generated file to
  manage for a single Android target); revisit if multi-platform ever matters.
