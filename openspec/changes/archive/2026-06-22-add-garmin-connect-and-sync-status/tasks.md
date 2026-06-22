## 1. Migration

- [x] 1.1 `task migrate:new NAME=add_sync_runs`; verify head is `050` first → this is `051`.
- [x] 1.2 `up.sql`: create `sync_runs` (id UUID pk, started_at TIMESTAMPTZ NOT NULL DEFAULT now(), finished_at TIMESTAMPTZ NULL, status TEXT NOT NULL DEFAULT 'running' CHECK status IN ('running','success','error'), window_from DATE NULL, window_to DATE NULL, error TEXT NULL, created_at/updated_at) + index on `(started_at DESC)`.
- [x] 1.3 `down.sql`: drop the table.

## 2. Backend package (internal/garminsyncstatus)

- [x] 2.1 `types.go`: `Status` enum (`running|success|error`) + `ValidCloseStatus` (only `success|error` for PATCH); `SyncRun` struct (dates as YYYY-MM-DD strings via to_char, timestamps as time.Time); `SyncStatus` response struct (`latest`, `last_successful_at`, `is_stale`).
- [x] 2.2 `repo.go`: `Insert` (open run), `Close` (set status/finished_at/error by id, ErrNotFound on miss), `Latest` (newest by started_at), `LastSuccessfulAt` (newest finished_at where status=success).
- [x] 2.3 `service.go`: open/close validation (`status_invalid` for a close outside success|error); `Status()` composes latest + last_successful_at + derived `is_stale` (no success within the threshold const, default 26h).
- [x] 2.4 `handlers.go`: `POST /garmin/sync-runs`, `PATCH /garmin/sync-runs/:id`, `GET /garmin/sync-status`; swag annotations.

## 3. Wiring + guards

- [x] 3.1 In `httpserver`: construct the service (garmin-enabled gate → `503 garmin_disabled` like garminauth); register routes.
- [x] 3.2 Guard the two write routes to the `garmin` identity (`auth.ClientFromContext != ClientGarmin → 403 forbidden`); leave `GET /garmin/sync-status` open to authenticated callers (mobile + agent).
- [x] 3.3 `task swag`.

## 4. MCP tool

- [x] 4.1 `internal/agenttools`: add a read-only `garmin_sync_status` tool → `GET /garmin/sync-status` (TierRead, MCP-exposed); forward verbatim.
- [x] 4.2 Bump `mcp_integration_test` expected-tools list + the schema golden (new tool).

## 5. Garmin bridge (apps/garmin-bridge)

- [x] 5.1 Wrap `POST /sync`: before fetching, `POST /garmin/sync-runs` with the rolling window → capture `id`.
- [x] 5.2 On success, `PATCH /garmin/sync-runs/{id}` `{status:"success"}`; on exception, `{status:"error", error:<short>}`.
- [x] 5.3 Make run reporting best-effort — a reporting failure logs but never aborts the data sync; cover with a bridge test/mocked backend.

## 6. Companion app (apps/companion)

- [x] 6.1 `domain/garmin.dart`: `GarminSyncStatus` (+ fromJson) and a connect-state enum.
- [x] 6.2 `state/garmin_provider.dart`: a `Notifier` login state machine (idle → triggering → awaiting_mfa → submitting_mfa → connected|error) driving `/garmin/login` + `/garmin/login/mfa`; and an `AsyncNotifier<GarminSyncStatus?>` read of `/garmin/sync-status` (stale-while-revalidate, mirroring the Train screen).
- [x] 6.3 `ui/garmin/garmin_connect_sheet.dart`: connect action + MFA code field + inline error mapping (`mfa_invalid`/`bad_credentials`/`locked_out`); never collects email/password.
- [x] 6.4 Sync-status card (last-synced relative time / syncing… / failed / not-configured on 503).
- [x] 6.5 Add a "Garmin" section to `ui/settings/settings_sheet.dart` opening the connect sheet + showing the sync-status card.
- [x] 6.6 Repository methods for the three calls (auth/baseUrl injected by the existing interceptor).

## 7. Tests + verification

- [x] 7.1 Backend: open→close(success); close(error) stores message; latest vs last_successful independence; no-runs returns nulls + is_stale=true; non-garmin write → 403; unknown PATCH → 404; invalid close status → 400; unconfigured → 503.
- [x] 7.2 MCP: `garmin_sync_status` issues exactly one GET; expected-tools list matches.
- [x] 7.3 `task test` (garminsyncstatus + agenttools green), `task vet`, `task swag` clean.
- [x] 7.4 Companion: widget/state test for the login state machine (needs_mfa branch, mfa error) and the sync-status card states, per the app's existing test setup.
