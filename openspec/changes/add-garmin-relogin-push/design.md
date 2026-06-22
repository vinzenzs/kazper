## Context

Garmin auth lapses today are silent. The garmin-bridge holds the credentials and, on an auth failure, clears the backend's opaque token (`DELETE /garmin/token`) and closes its `sync_runs` row `error`. `GET /garmin/sync-status` exposes `is_stale`, but nothing pushes the user — they must open the app and notice. The repo is the Go/Gin backend; the Flutter `mobile-companion` and the `garmin-bridge` are separate processes described by specs that live here. There is no push infrastructure yet (no FCM config, no device-token store).

Two forks were resolved with the user up front:
1. **How the backend learns relogin is needed** → *infer from token state*: on a sync-run error-close, check whether the Garmin token is absent. No new bridge contract.
2. **Delivery transport** → *FCM HTTP v1* with a service-account credential, opt-in/gated exactly like the existing Garmin integration.

## Goals / Non-Goals

**Goals:**
- A new `internal/push` capability: device-token registration, an opt-in FCM HTTP v1 sender, and the relogin-needed notification with once-per-outage latching.
- Trigger the relogin push from the existing sync-run error-close path without changing that endpoint's request/response contract or the bridge.
- Keep push fully optional — unconfigured = silent no-op, `503 push_disabled` on operations that require the sender; never fail a Garmin close because of push.

**Non-Goals:**
- No general notification framework (no per-event preferences, no scheduling, no iOS/APNs — Android only, single user).
- No MCP tools for push (device-specific, not an agent concern).
- No retry queue / delivery guarantees beyond best-effort fan-out + pruning dead tokens. A missed push is backstopped by `is_stale`.
- No change to how the bridge detects auth failure or clears the token.

## Decisions

### D1 — Trigger by inferring from token state, hooked into the sync-run close
On `PATCH /garmin/sync-runs/{id}` → `status=error`, the sync-status service asks a `garminTokenPresence` port "is a token stored?" If absent → call `pushSvc.NotifyReloginNeeded(ctx)`. On `status=success` → `pushSvc.ClearReloginLatch(ctx)`.
- *Why over a bridge-tagged error_kind:* keeps the bridge contract untouched and the backend free of Garmin protocol knowledge — it only reads token presence, which it already owns. The bridge already clears the token on auth failure, so absence is a faithful proxy for "relogin needed."
- *Wiring:* mirror the existing `mealsSvc.SetWorkoutsRepo(...)` cross-injection in `httpserver/server.go`. The sync-status service gets two injected ports (`SetReloginNotifier`, `SetGarminTokenPresence`); both default to nil/no-op so the package stays independently testable. Side-effects run after the row is committed and their errors are logged, never returned.

### D2 — Latch lives in the push package as a singleton row
A single-row `relogin_latch` (e.g. a one-row table or a `kv`-style row) stores `notified bool` / `notified_at`. `NotifyReloginNeeded` is idempotent: send + set only when unset. `ClearReloginLatch` resets it.
- *Why a latch over reading sync-run history:* we don't store per-run token presence, so "did the previous run already represent this outage?" isn't derivable from `sync_runs` alone. An explicit latch is the honest, simple state. Single-user → one row.
- *Reset points:* `PUT /garmin/token` (re-auth succeeded) and success-close (sync recovered). Both wired the same cross-injection way; garmin-auth gets a `SetReloginLatchClearer` port.

### D3 — FCM HTTP v1, service-account JWT → OAuth2 access token, no Admin SDK
The sender mints a short-lived access token by signing the service-account JWT (`https://www.googleapis.com/auth/firebase.messaging` scope) and exchanging it at Google's token endpoint, caching it until near expiry, then `POST`s `https://fcm.googleapis.com/v1/projects/{FCM_PROJECT_ID}/messages:send` once per token.
- *Why not Firebase Admin SDK:* it's a heavy dependency for one call; the JWT-exchange + single POST is small and isolated, matching the repo's thin-client style (cf. the `off` client). Uses `golang.org/x/oauth2/google` for the credential/token source if already vendored, else a minimal hand-rolled signer.
- *Token shape:* `notification` (title/body) for the tray + `data:{action:"garmin_relogin"}` for deep-linking.
- *Dead-token pruning:* a `404`/`UNREGISTERED` from FCM deletes that `push_tokens` row inline; other errors are logged and skipped, remaining tokens still attempted.

### D4 — Config gating identical to Garmin
`FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON` (inline JSON or path). Push enabled iff both set. `FCM_SERVICE_ACCOUNT_JSON` validated to parseable service-account JSON at load (named error otherwise), redacted in `redact()` alongside the Garmin secrets. When disabled, the push service is constructed in a no-op mode and endpoints return `503 push_disabled`.

### D5 — Registration is mobile-only and survives disabled push
`POST /push/tokens` (upsert by token) and `DELETE /push/tokens` are restricted to the `mobile` identity. Registration is accepted even when the sender is unconfigured, so enabling FCM later delivers without re-pairing. Package shape follows the standard `types.go / repo.go / service.go / handlers.go / *_test.go`; `Register(rg)` wired in `server.go`; `task swag` regenerated.

## Risks / Trade-offs

- **Token-absence is a proxy, not a guarantee, that relogin is needed** → If the bridge ever errors a sync *and* the token happens to be absent for an unrelated reason, the user gets a benign "re-login" nudge. Acceptable: re-login is idempotent and the bridge's own behavior (clear-on-auth-failure) makes false positives unlikely.
- **No delivery guarantee / FCM outage** → A dropped push leaves the user uninformed; mitigated by keeping `is_stale` as the always-available backstop (asserted in the mobile-companion spec) and by not latching on a *failed* send only when at least the latch set succeeds (send best-effort, latch set regardless to avoid spamming — re-notification still happens at the next outage after a reset).
- **Multiple stale tokens accumulate** → mitigated by inline pruning on `UNREGISTERED`, and tokens are cheap.
- **Side-effect coupling in the close path** → mitigated by running pushes post-commit, swallowing their errors, and keeping the ports nil-defaulted so sync-status and garmin-auth tests don't need push.
- **Secret handling** → service-account JSON is a high-value secret; redaction is covered by a config requirement + scenario, mirroring the existing token-redaction tests.

## Migration Plan

1. Add migration (next sequential number — verify head, currently ~mid-040s) creating `push_tokens` (+ unique on `token`) and the singleton `relogin_latch` row/table.
2. Ship backend with push **unconfigured** by default → entirely inert; no behavior change for existing deployments.
3. Configure `FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON`, deploy the companion build that registers its token, then verify an induced relogin (clear token + error-close) delivers exactly one push and that `PUT /garmin/token` silences it.
- *Rollback:* unset the two FCM keys → sender goes inert; the migration is additive and safe to leave in place.

## Open Questions

- Should a registered-but-stale latch also auto-clear after a max age (e.g. if neither success nor re-auth happens for N days) to allow a re-nudge? Default: no — recovery events are the only reset; revisit if real use shows a single push is too easy to miss.
- Body copy / localization of the notification — deferred to implementation (single-user, English).
