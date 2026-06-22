## Why

When the garmin-bridge's auth token expires, the bridge clears it (`DELETE /garmin/token`) and the next sync fails — but nothing tells the user. Garmin data silently goes stale until the user happens to open the app, notices, and re-runs the login flow. The whole point of a single-user fueling system is that the data is current; a silent auth lapse quietly poisons every downstream computation (energy availability, hydration, fueling). The user needs a *push* the moment relogin is required, not a passive `is_stale` flag they have to go looking for.

## What Changes

- **New push-notification capability.** Add an opt-in FCM (HTTP v1) sender, gated on configuration exactly like the Garmin integration. When unconfigured the surface is a no-op and its endpoints return `503 push_disabled`.
- **Push-token registration.** The mobile companion registers its FCM registration token via `POST /push/tokens` (upsert) and can drop it via `DELETE /push/tokens`. Restricted to the `mobile` identity; tokens stored in a dedicated `push_tokens` table. Not mirrored to MCP (device-specific, not an agent concern).
- **Relogin push trigger (backend-inferred).** When a sync run is closed `status=error` (`PATCH /garmin/sync-runs/{id}`) *and* the Garmin token is absent (`404`), the backend infers "relogin needed" and sends a single push to every registered token. A latch prevents re-notifying on every subsequent failed sync within the same outage.
- **Latch reset.** Storing a fresh token (`PUT /garmin/token`) or closing a sync run `status=success` clears the latch, so a future lapse notifies again but a recovered session goes quiet.
- **Mobile companion handling.** The app registers its token on pairing/launch and, on receiving the relogin notification, deep-links to the existing Garmin re-authentication flow (`POST /garmin/login` → `/garmin/login/mfa`).
- **Config.** New opt-in keys `FCM_PROJECT_ID` and `FCM_SERVICE_ACCOUNT_JSON` (inline JSON or path), redacted in config dumps.

## Capabilities

### New Capabilities
- `push-notifications`: registration of device push tokens, an opt-in FCM HTTP v1 sender gated on config, and the relogin-needed notification triggered by Garmin sync failure with an absent token (including the once-per-outage latch).

### Modified Capabilities
- `garmin-sync-status`: closing a sync run `error` with an absent Garmin token triggers a relogin push; closing `success` clears the relogin latch.
- `garmin-auth`: storing a token (`PUT /garmin/token`) clears the relogin latch.
- `mobile-companion`: the app registers its FCM token and handles the relogin notification by deep-linking into the Garmin re-authentication flow.
- `config`: new opt-in `FCM_PROJECT_ID` / `FCM_SERVICE_ACCOUNT_JSON` keys, gating and redaction.

## Impact

- **New package** `internal/push/` (types, repo, service, FCM HTTP v1 client, handlers) following the per-capability shape.
- **New migration** adding `push_tokens` and a singleton relogin-latch row (next sequential number — verify the current head before committing).
- **Wiring** in `internal/httpserver/server.go`: instantiate push, cross-inject a relogin notifier + Garmin-token presence check into the sync-status service, and the latch-clear into garmin-auth's token store.
- **Config** `internal/config/config.go`: two new keys + redaction.
- **Docs**: `task swag` regenerated after the new handlers; `.env` examples updated.
- **External dependency**: a Google OAuth2 access-token mint for FCM HTTP v1 (service-account JWT exchange) — a small, isolated client; no Firebase Admin SDK pulled in.
- Single-user assumption preserved: one latch row, but multiple device tokens supported (fan-out send).
