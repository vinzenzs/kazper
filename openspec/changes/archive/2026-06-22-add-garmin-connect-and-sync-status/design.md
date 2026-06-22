## Context

The Garmin integration is split across three processes: the **garmin-bridge** (a Python
app that owns all Garmin-protocol knowledge — SSO login, MFA, the `/sync` job), the **Go
backend** (which proxies the bridge's login to authenticated callers and stores the
encrypted token via `garminauth`), and the **clients** (companion app under the `mobile`
identity, coach under `agent`, bridge under `garmin`). The bridge already does interactive
login with MFA (`begin_login`/`resume_login` in `garmin_client.py`) using credentials from
its own config, and the backend already proxies it (`internal/garmincontrol`:
`POST /garmin/login`, `POST /garmin/login/mfa`). There is **no** record of sync runs:
`devices.last_sync_at` is the watch's field, and the rolling `/sync` job writes data but
logs nothing about itself.

Two decisions from exploration shape this design: credentials stay **bridge-side** (the app
only triggers login + relays MFA), and sync freshness is tracked **explicitly** (a
`sync_runs` log the bridge reports to), not inferred from data timestamps.

## Goals / Non-Goals

**Goals:**
- Let the companion app complete a Garmin (re)login, including MFA, with no SSH.
- Give the app + coach a truthful answer to "is my Garmin data current?", including
  in-progress and failed syncs.
- Reuse the existing login proxy verbatim; add only the sync-run surface server-side.

**Non-Goals:**
- **No credential entry in the app.** Email/password stay in the bridge config; the app
  never sees or transmits them. (Chosen explicitly — keeps credentials off the phone and
  out of the backend.)
- **No change to the login/MFA endpoints.** They already exist and are sufficient.
- **No per-record sync provenance** beyond the run log (we don't tag each workout with a
  `sync_run_id`); the run log stands alone.
- No retry/scheduling logic for failed syncs — surfacing the failure is enough for now.

## Decisions

### 1. `sync_runs` as an explicit, bridge-reported log

```
sync_runs
  id           UUID pk
  started_at   TIMESTAMPTZ NOT NULL DEFAULT now()
  finished_at  TIMESTAMPTZ NULL                      -- null while running
  status       TEXT NOT NULL CHECK (running|success|error) DEFAULT 'running'
  window_from  DATE NULL                             -- rolling window the run covered
  window_to    DATE NULL
  error        TEXT NULL                             -- set when status=error
  created_at / updated_at
index (started_at DESC)                              -- "latest run" is the dominant read
```

The bridge **opens** a run (`POST /garmin/sync-runs` with the window) before fetching and
**closes** it (`PATCH /garmin/sync-runs/{id}` with `success`/`error`) after. A run left
`running` past a sensible age is treated as stale/aborted by the reader (the bridge process
died mid-sync) — surfaced as such rather than mutated.

**Alternative considered — infer last-sync from `max(updated_at)` over garmin-sourced
rows.** Rejected (the user's call): it can't distinguish "sync ran, nothing new" from "sync
failed", and shows no in-progress or error state — exactly the trust questions this feature
exists to answer.

### 2. Identity split on the endpoints (reuse the garminauth guard)

| Endpoint                       | Identity        | Why                                            |
|--------------------------------|-----------------|------------------------------------------------|
| `POST /garmin/sync-runs`       | `garmin` only   | only the bridge records runs (403 otherwise)   |
| `PATCH /garmin/sync-runs/{id}` | `garmin` only   | "                                              |
| `GET /garmin/sync-status`      | `mobile`+`agent`| the app + coach read freshness                 |

The write guard is the exact pattern `garminauth` uses
(`auth.ClientFromContext(c) != auth.ClientGarmin → 403`), and like the token endpoints the
whole surface returns `503 garmin_disabled` when the integration is unconfigured.
`GET /garmin/sync-status` is a normal authenticated read (no garmin-only restriction) so
the mobile app can call it.

### 3. `GET /garmin/sync-status` shape — latest run + last success

```json
{
  "latest": { "id", "status", "started_at", "finished_at", "window_from", "window_to", "error" },
  "last_successful_at": "2026-06-22T05:03:00Z",   // finished_at of the newest success, or null
  "is_stale": false                                 // derived: no success within a threshold
}
```

`last_successful_at` is a separate query (newest `status=success`) so a *failed* latest run
still shows when data was last good. `is_stale` is a server-derived convenience
(no success within ~26 h, covering a daily sync + slack) so the app needn't re-encode the
rule. Both reads are pure — no synthesis, matching the project's read-aggregator style.

### 4. App login flow drives the existing proxy (state machine, no new endpoint)

The connect flow is a `Notifier` state machine mirroring `scan_provider`:
`idle → triggering → (awaiting_mfa | connected | error) → submitting_mfa → (connected | error)`.
`triggering` posts `/garmin/login`; `{needs_mfa:true}` → `awaiting_mfa` (show 6-digit
field); `{logged_in:true}` → `connected`. `submitting_mfa` posts `/garmin/login/mfa`. The
bridge's typed errors (`mfa_invalid`, `bad_credentials`, `locked_out`) map to inline
messages. The sync-status card is a separate `AsyncNotifier<GarminSyncStatus?>` read,
stale-while-revalidate like the Train screen.

## Risks / Trade-offs

- **[A `running` row never closed (bridge crash)]** → The reader derives staleness from
  `last_successful_at` and ages out a stuck `running` run in the response rather than
  trusting `status` blindly; optionally the bridge closes any orphaned `running` row on its
  next start.
- **[Login proxy needs the bridge reachable]** → Already true for today's sync; the
  endpoints already return `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset, so the
  app degrades to "Garmin unavailable".
- **[Single-user assumption]** → `sync_runs` is unscoped (one athlete), consistent with the
  rest of the schema.
- **[Bridge is Python, separate workstream]** → The bridge change is small (two HTTP calls
  wrapping `/sync`) and specced in the `garmin-bridge` delta; backend works without it but
  the log stays empty until the bridge reports.

## Migration Plan

1. `task migrate:new NAME=add_sync_runs` — verify head is `050` first; this is `051`.
2. `up.sql`: create `sync_runs` with the CHECK + `started_at DESC` index. `down.sql`: drop.
3. Backend `internal/garminsyncstatus/`: types/repo/service/handlers; wire in `httpserver`
   (garmin-disabled gate + garmin-only guard on writes); `task swag`.
4. MCP `garmin_sync_status` read tool; bump `mcp_integration_test`.
5. Bridge: open/close a run around `/sync`.
6. Companion: connect sheet + sync-status card + settings section.

## Open Questions

- **Staleness threshold** for `is_stale` — 26 h (daily sync + slack) is the default; revisit
  if `SYNC_LOOKBACK_DAYS`/cadence changes make it noisy. Confirm at apply.
- **Orphaned `running` cleanup** — derive-in-reader only, or also have the bridge close
  stale runs on startup? Lean reader-only first (simpler); add bridge cleanup if stuck rows
  prove annoying.
