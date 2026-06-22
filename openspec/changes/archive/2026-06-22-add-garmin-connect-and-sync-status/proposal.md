## Why

Two Garmin-connection gaps surface on the phone. First, when the stored Garmin token
expires (or on first setup), re-authentication today means SSHing to trigger the bridge
login and reading the MFA prompt out-of-band — there's no way to do it from the companion
app. The backend already proxies the bridge's interactive login (`POST /garmin/login` →
`{needs_mfa}`, `POST /garmin/login/mfa`), so the app just needs to drive that flow.
Second, the app can't answer the most basic trust question — *"is my Garmin data current?"*
There is no record anywhere of when a sync last ran: `devices.last_sync_at` is the watch's
own field, and the backend keeps no sync-run history. So a stale dashboard is
indistinguishable from a fresh one.

## What Changes

- **Garmin connect from the app (no new backend login work).** The companion app drives
  the *existing* login proxy: a "Connect Garmin" action calls `POST /garmin/login`; on
  `{needs_mfa: true}` it prompts for the 6-digit code and calls `POST /garmin/login/mfa`.
  Credentials stay in the bridge's own config (the app never collects email/password) —
  the app only triggers login and relays the MFA code.
- **Explicit sync-run tracking (net-new).** A `sync_runs` table records each bridge sync:
  `started_at`, `finished_at`, `status` (`running | success | error`), the rolling
  `window_from`/`window_to`, and an `error` message on failure. The garmin-bridge reports
  a run when it starts (`POST /garmin/sync-runs` → row with `status=running`) and on
  completion (`PATCH /garmin/sync-runs/{id}` → `success`/`error`). These writes are
  restricted to the `garmin` identity (same guard as the token endpoints).
- **Sync-status read for the app + coach.** `GET /garmin/sync-status` returns the latest
  run plus the `last_successful_at` timestamp, readable by the `mobile` and `agent`
  identities. Mirrored as a read-only MCP tool so the coach can tell the user "your last
  Garmin sync was 3 h ago."
- **Companion app UI.** A "Garmin" section in the settings sheet showing connection +
  last-sync state, a Garmin-connect flow (trigger → MFA entry → result), and a sync-status
  card ("Last synced 2 h ago" / "Sync failed" / "Syncing…").

## Capabilities

### New Capabilities
- `garmin-sync-status`: the `sync_runs` log, the bridge-facing record endpoints
  (`POST`/`PATCH /garmin/sync-runs`), and the `GET /garmin/sync-status` read + its MCP tool.

### Modified Capabilities
- `garmin-bridge`: the bridge SHALL report each `/sync` invocation to the backend — open a
  run before fetching and close it (success/error) after, so the backend's sync-run log
  reflects reality.
- `mobile-companion`: the app SHALL surface a Garmin connection + sync-status section —
  drive the existing login/MFA proxy and display `GET /garmin/sync-status`.

## Impact

- **Schema:** migration `051` — `sync_runs` table (head is `050`; verify before apply).
- **Backend code:** new `internal/garminsyncstatus/` package (types/repo/service/handlers);
  route registration in `httpserver`; a read MCP tool in `internal/agenttools`; bump the
  `mcp_integration_test` expected-tools list.
- **Garmin bridge** (`apps/garmin-bridge/`): wrap `/sync` to open/close a sync-run via the
  backend under the `garmin` identity.
- **Companion app** (`apps/companion/`): a Garmin connect sheet (login-trigger + MFA state
  machine, mirroring the `scan_provider` Notifier pattern), a sync-status `AsyncNotifier`
  read (mirroring the Train screen), and a settings-sheet "Garmin" section.
- **No backend login change** — `POST /garmin/login` + `/garmin/login/mfa` already exist
  and are consumed as-is (credentials remain bridge-side).
- **Docs:** `task swag` after the new handlers.
