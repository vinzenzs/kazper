## ADDED Requirements

### Requirement: The bridge reports each sync run to the backend

When the bridge runs `POST /sync` it SHALL record the run in the backend's sync-run log so
the backend has an authoritative history of when Kazper last pulled from Garmin. Before
fetching, the bridge SHALL open a run via `POST /garmin/sync-runs` (under the `GARMIN_API_TOKEN`
`garmin` identity) carrying the rolling window it is about to sync (`window_from`/`window_to`),
capturing the returned run `id`. After the sync completes it SHALL close the run via `PATCH
/garmin/sync-runs/{id}` with `status=success`; if the sync raises, it SHALL close the run with
`status=error` and a short, log-safe `error` message. Reporting failures (the backend
unreachable) SHALL NOT abort the sync itself — the data write is the primary job and run
reporting is best-effort.

#### Scenario: A successful sync opens and closes a run

- **WHEN** `POST /sync` runs and completes without error
- **THEN** the bridge has opened a `sync_runs` row (`status=running`) with the synced window and then closed it with `status=success`

#### Scenario: A failed sync closes the run as error

- **WHEN** `POST /sync` raises partway through (e.g. Garmin rate-limits)
- **THEN** the bridge closes the open run with `status=error` and a short error message

#### Scenario: Run reporting is best-effort

- **WHEN** the backend is unreachable when the bridge tries to open or close a run
- **THEN** the bridge logs the reporting failure but the sync's data writes still proceed (run reporting never aborts the sync)
