## 1. Config

- [x] 1.1 Add `FCM_PROJECT_ID` and `FCM_SERVICE_ACCOUNT_JSON` to `internal/config/config.go` (env bindings + struct fields), with a `PushEnabled()` helper that is true iff both are set.
- [x] 1.2 Validate `FCM_SERVICE_ACCOUNT_JSON` resolves to parseable service-account JSON when set (inline JSON or readable file path); return an error naming the variable otherwise.
- [x] 1.3 Redact `FCM_SERVICE_ACCOUNT_JSON` in `redact()` alongside the Garmin secrets.
- [x] 1.4 Config tests: disabled when unset, enabled when both set, invalid JSON rejected, secret redacted in diagnostics.

## 2. Migration

- [x] 2.1 `task migrate:new NAME=add_push_tokens_and_relogin_latch` (verify head is `051`; new pair is `052`).
- [x] 2.2 Up: create `push_tokens` (`id` UUID PK, `token` TEXT NOT NULL, `platform` TEXT NOT NULL DEFAULT 'android', `created_at`, `updated_at`) with a UNIQUE index on `(token)`; create the singleton `relogin_latch` (one-row table with `notified BOOLEAN NOT NULL DEFAULT false`, `notified_at TIMESTAMPTZ NULL`) and seed the single row. Down: drop both.

## 3. push package (`internal/push/`)

- [x] 3.1 `types.go` — `PushToken` row struct + JSON tags; relogin latch state struct.
- [x] 3.2 `repo.go` — token upsert-by-token, delete-by-token, list-all; latch read / set-notified / clear, against `store.Querier`.
- [x] 3.3 `fcm.go` — FCM HTTP v1 client: mint+cache OAuth2 access token from the service-account credential (firebase.messaging scope), `messages:send` per token; map `404`/`UNREGISTERED` to a prune signal. No-op client when push is disabled.
- [x] 3.4 `service.go` — `RegisterToken`, `RemoveToken`; `NotifyReloginNeeded` (latched: send to all tokens + set latch only when unset, prune dead tokens, swallow send errors); `ClearReloginLatch`; sentinel errors mapping to API error codes (`push_disabled`).
- [x] 3.5 `handlers.go` — `POST /push/tokens`, `DELETE /push/tokens` with swag annotations, restricted to the `mobile` identity; `Register(rg *gin.RouterGroup)`.
- [x] 3.6 Per-handler integration tests against testcontainers Postgres (register upsert idempotency, delete, 403 for agent/garmin identities).

## 4. Sender + latch unit coverage

- [x] 4.1 Unit-test the latch logic in `service.go`: first relogin notifies + sets latch; repeat while set is a no-op; clear resets; disabled push is a silent no-op.
- [x] 4.2 Test dead-token pruning: an `UNREGISTERED` response deletes that row, other tokens still attempted.

## 5. Trigger wiring

- [x] 5.1 Add a `ReloginNotifier` port (`NotifyReloginNeeded`) and a `GarminTokenPresence` port (`HasToken`) to the garmin-sync-status service via `SetReloginNotifier` / `SetGarminTokenPresence`, both nil-defaulted to no-op.
- [x] 5.2 In the sync-run close handler: on `status=error` with no stored token → call `NotifyReloginNeeded`; on `status=success` → `ClearReloginLatch`. Run side-effects post-commit; log (never return) their errors so the close contract is unchanged.
- [x] 5.3 Add a `ReloginLatchClearer` port to the garmin-auth service via `SetReloginLatchClearer` (nil-defaulted); call it on token store (`PUT/POST /garmin/token`).
- [x] 5.4 Wire all instantiations + cross-injections in `internal/httpserver/server.go` (`push` package construction, register routes, inject notifier/presence into sync-status, inject latch-clearer into garmin-auth).

## 6. Integration tests for the trigger

- [x] 6.1 garmin-sync-status: error-close with token absent invokes the notifier; error-close with token present does not; success-close clears the latch. Use a fake notifier/presence to assert calls (push package has its own send tests).
- [x] 6.2 garmin-auth: storing a token clears the latch; a latch-clear error does not fail the token store.

## 7. Docs + final

- [x] 7.1 `task swag` to regenerate `docs/` after the new handlers.
- [x] 7.2 Update `.env.local` / README example env with the two opt-in FCM keys (documented as optional).
- [x] 7.3 `task vet` and `task test` green; single-package runs for `push`, `garminsyncstatus`, `garminauth`, `config`.
