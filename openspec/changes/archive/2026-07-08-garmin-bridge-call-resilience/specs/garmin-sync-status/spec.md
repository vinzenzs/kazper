## MODIFIED Requirements

### Requirement: Sync runs are recorded in a dedicated log

The system SHALL persist Garmin sync runs in a `sync_runs` table, one row per bridge sync
invocation, holding `started_at`, a nullable `finished_at`, a `status` from the set
`running | success | error | partial` (default `running`), the rolling window the run covered
(`window_from`, `window_to`, both nullable dates), a nullable `error` message, a nullable
`summary` (a compact JSON roll-up of a multi-day/long job — e.g. a backfill's `days_total`,
`days_ok`, `days_failed`, and per-day results), and audit timestamps. The `partial` status
denotes a run that completed with one or more isolated per-day failures (distinct from
`error`, a run-level failure such as auth), and `summary` carries the detail that a
non-blocking job can no longer return in its synchronous response body. The table is the
backend's authoritative record of *when Kazper last pulled from Garmin* — distinct from
`devices.last_sync_at`, which is the watch's own field. The whole sync-run surface is gated on
the Garmin integration being configured: when it is not, the endpoints SHALL return
`503 garmin_disabled`.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration is applied
- **THEN** a `sync_runs` table exists with `status` CHECK-constrained to `running|success|error|partial`, `status` defaulting to `running`, `finished_at`/`window_from`/`window_to`/`error`/`summary` nullable, and an index on `started_at` descending

#### Scenario: A partial run carries its roll-up

- **WHEN** a background backfill completes with one or more failed days
- **THEN** its run is closed `status=partial` with a `summary` roll-up reporting `days_total`/`days_ok`/`days_failed`

### Requirement: Sync status is readable by the app and coach

The system SHALL expose `GET /garmin/sync-status` returning `latest` (the most recent run
by `started_at`, or null when none exists), `last_successful_at` (the `finished_at` of the
newest `status=success` run, or null), and a derived boolean `is_stale` (true when no
successful run has completed within the staleness threshold). The endpoint SHALL accept an
optional `run_id` query parameter; when supplied it SHALL return that specific run (as
`latest`), so a caller holding the `run_id` from a `202` backfill trigger can poll a
particular run even while a concurrent daily sync opens newer runs (`404` when the id is
unknown). Each returned run SHALL include its `summary` roll-up when present. Only
`status=success` counts toward `last_successful_at` and `is_stale`; a `partial` or `error`
run is not a success. The read SHALL be available to the `mobile` and `agent` identities (it
is not garmin-only) and SHALL perform no synthesis — it returns the stored runs verbatim plus
the two derived fields. When the integration is unconfigured it SHALL return `503 garmin_disabled`.

#### Scenario: Latest run and last success are reported independently

- **WHEN** the newest run is `status=error` but an earlier run succeeded
- **THEN** `latest` reflects the error run AND `last_successful_at` carries the earlier success's `finished_at`

#### Scenario: A backfill run is pollable by run id

- **WHEN** a client `GET`s `/garmin/sync-status?run_id=<id>` for the run returned by a `202` backfill trigger
- **THEN** the response's `latest` is that run — reporting `running` while the background replay proceeds and its terminal `status`/`summary` once it completes — regardless of any newer daily-sync run
- **AND** an unknown `run_id` returns `404`

#### Scenario: A partial run does not count as a success

- **WHEN** the newest run is `status=partial`
- **THEN** it is reflected in `latest` but does NOT update `last_successful_at`, and `is_stale` is derived from the last `success` run only

#### Scenario: No runs yet

- **WHEN** no sync run has ever been recorded and the client `GET`s `/garmin/sync-status`
- **THEN** the response is `200` with `latest:null`, `last_successful_at:null`, and `is_stale:true`

#### Scenario: Freshly synced is not stale

- **WHEN** the newest successful run finished within the staleness threshold
- **THEN** `is_stale` is false

#### Scenario: The coach can read sync freshness via MCP

- **WHEN** the agent calls the `garmin_sync_status` tool
- **THEN** the MCP server issues exactly one `GET /garmin/sync-status` and returns the response verbatim
