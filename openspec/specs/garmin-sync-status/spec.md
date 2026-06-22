# garmin-sync-status Specification

## Purpose
The backend's authoritative record of when Kazper last pulled from Garmin — distinct from `devices.last_sync_at` (the watch's own field). The garmin-bridge opens a `sync_runs` row before each `/sync` and closes it `success`/`error` after; the companion app and coach read `GET /garmin/sync-status` to answer "is my Garmin data current?", including in-progress and failed syncs. A storage + read primitive: no synthesis. The whole surface is gated `503 garmin_disabled` when the integration is unconfigured, and the record endpoints are restricted to the `garmin` identity.

## Requirements
### Requirement: Sync runs are recorded in a dedicated log

The system SHALL persist Garmin sync runs in a `sync_runs` table, one row per bridge sync
invocation, holding `started_at`, a nullable `finished_at`, a `status` from the set
`running | success | error` (default `running`), the rolling window the run covered
(`window_from`, `window_to`, both nullable dates), a nullable `error` message, and audit
timestamps. The table is the backend's authoritative record of *when Kazper last pulled
from Garmin* — distinct from `devices.last_sync_at`, which is the watch's own field. The
whole sync-run surface is gated on the Garmin integration being configured: when it is not,
the endpoints SHALL return `503 garmin_disabled`.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration is applied
- **THEN** a `sync_runs` table exists with `status` CHECK-constrained to `running|success|error`, `status` defaulting to `running`, `finished_at`/`window_from`/`window_to`/`error` nullable, and an index on `started_at` descending

### Requirement: The bridge opens and closes a sync run via the garmin identity

The system SHALL expose `POST /garmin/sync-runs` that records a new run (status `running`,
optional `window_from`/`window_to`) and returns it with its generated `id`, and `PATCH
/garmin/sync-runs/{id}` that closes a run by setting `status` to `success` or `error`,
stamping `finished_at`, and recording an `error` message when failed. Both endpoints SHALL
be restricted to the `garmin` identity — any other identity receives `403 forbidden` — and
SHALL return `503 garmin_disabled` when the integration is unconfigured. `PATCH` SHALL
return `404 sync_run_not_found` for an unknown id and reject a `status` outside
`success|error` with `400 status_invalid`.

#### Scenario: Bridge opens a run

- **WHEN** the bridge `POST`s `/garmin/sync-runs` with `{"window_from":"2026-06-20","window_to":"2026-06-22"}` under the `garmin` identity
- **THEN** the response is `201` with a row carrying a generated `id`, `status:"running"`, `finished_at:null`, and the supplied window

#### Scenario: Bridge closes a run as success

- **WHEN** the bridge `PATCH`es `/garmin/sync-runs/{id}` with `{"status":"success"}`
- **THEN** the run's `status` becomes `success` and `finished_at` is stamped

#### Scenario: Bridge closes a run as error with a message

- **WHEN** the bridge `PATCH`es `{"status":"error","error":"garmin 429 rate limited"}`
- **THEN** the run's `status` becomes `error`, `finished_at` is stamped, and the `error` text is stored

#### Scenario: A non-garmin identity cannot record runs

- **WHEN** the `mobile` or `agent` identity calls `POST /garmin/sync-runs` or `PATCH /garmin/sync-runs/{id}`
- **THEN** the response is `403 forbidden`

#### Scenario: Closing an unknown run is a 404

- **WHEN** the bridge `PATCH`es an id that does not exist
- **THEN** the response is `404 sync_run_not_found`

#### Scenario: An invalid close status is rejected

- **WHEN** the bridge `PATCH`es `{"status":"running"}` or any value outside `success|error`
- **THEN** the response is `400 status_invalid`

### Requirement: Sync status is readable by the app and coach

The system SHALL expose `GET /garmin/sync-status` returning `latest` (the most recent run
by `started_at`, or null when none exists), `last_successful_at` (the `finished_at` of the
newest `status=success` run, or null), and a derived boolean `is_stale` (true when no
successful run has completed within the staleness threshold). The read SHALL be available to
the `mobile` and `agent` identities (it is not garmin-only) and SHALL perform no synthesis —
it returns the stored runs verbatim plus the two derived fields. When the integration is
unconfigured it SHALL return `503 garmin_disabled`.

#### Scenario: Latest run and last success are reported independently

- **WHEN** the newest run is `status=error` but an earlier run succeeded
- **THEN** `latest` reflects the error run AND `last_successful_at` carries the earlier success's `finished_at`

#### Scenario: No runs yet

- **WHEN** no sync run has ever been recorded and the client `GET`s `/garmin/sync-status`
- **THEN** the response is `200` with `latest:null`, `last_successful_at:null`, and `is_stale:true`

#### Scenario: Freshly synced is not stale

- **WHEN** the newest successful run finished within the staleness threshold
- **THEN** `is_stale` is false

#### Scenario: The coach can read sync freshness via MCP

- **WHEN** the agent calls the `garmin_sync_status` tool
- **THEN** the MCP server issues exactly one `GET /garmin/sync-status` and returns the response verbatim

