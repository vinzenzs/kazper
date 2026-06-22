## MODIFIED Requirements

### Requirement: The bridge opens and closes a sync run via the garmin identity

The system SHALL expose `POST /garmin/sync-runs` that records a new run (status `running`,
optional `window_from`/`window_to`) and returns it with its generated `id`, and `PATCH
/garmin/sync-runs/{id}` that closes a run by setting `status` to `success` or `error`,
stamping `finished_at`, and recording an `error` message when failed. Both endpoints SHALL
be restricted to the `garmin` identity — any other identity receives `403 forbidden` — and
SHALL return `503 garmin_disabled` when the integration is unconfigured. `PATCH` SHALL
return `404 sync_run_not_found` for an unknown id and reject a `status` outside
`success|error` with `400 status_invalid`.

On closing a run, the system SHALL drive the Garmin relogin push side-effect: when a run is
closed `status=error` AND the stored Garmin token is absent, the system SHALL invoke the
relogin-needed notification (which is itself latched and gated on push configuration —
see the `push-notifications` capability); when a run is closed `status=success`, the system
SHALL clear the relogin latch. These side-effects SHALL NOT change the endpoint's response
contract: a push-send failure or unconfigured push surface SHALL NOT fail the close, which
still returns the updated run.

#### Scenario: Bridge opens a run

- **WHEN** the bridge `POST`s `/garmin/sync-runs` with `{"window_from":"2026-06-20","window_to":"2026-06-22"}` under the `garmin` identity
- **THEN** the response is `201` with a row carrying a generated `id`, `status:"running"`, `finished_at:null`, and the supplied window

#### Scenario: Bridge closes a run as success

- **WHEN** the bridge `PATCH`es `/garmin/sync-runs/{id}` with `{"status":"success"}`
- **THEN** the run's `status` becomes `success` and `finished_at` is stamped
- **AND** the Garmin relogin latch is cleared

#### Scenario: Bridge closes a run as error with a message

- **WHEN** the bridge `PATCH`es `{"status":"error","error":"garmin 429 rate limited"}`
- **THEN** the run's `status` becomes `error`, `finished_at` is stamped, and the `error` text is stored

#### Scenario: Error close with an absent token triggers a relogin push

- **WHEN** the bridge `PATCH`es a run to `status=error` and the Garmin token is absent
- **THEN** the run is closed as error AND a relogin-needed notification is invoked (subject to its own latch and push configuration)

#### Scenario: Error close with the token still present triggers no push

- **WHEN** the bridge `PATCH`es a run to `status=error` and the Garmin token is still stored
- **THEN** the run is closed as error AND no relogin notification is invoked

#### Scenario: A push or latch failure does not fail the close

- **WHEN** closing a run would trigger a push but the push send errors (or push is unconfigured)
- **THEN** the close still returns the updated run with its new status

#### Scenario: A non-garmin identity cannot record runs

- **WHEN** the `mobile` or `agent` identity calls `POST /garmin/sync-runs` or `PATCH /garmin/sync-runs/{id}`
- **THEN** the response is `403 forbidden`

#### Scenario: Closing an unknown run is a 404

- **WHEN** the bridge `PATCH`es an id that does not exist
- **THEN** the response is `404 sync_run_not_found`

#### Scenario: An invalid close status is rejected

- **WHEN** the bridge `PATCH`es `{"status":"running"}` or any value outside `success|error`
- **THEN** the response is `400 status_invalid`
