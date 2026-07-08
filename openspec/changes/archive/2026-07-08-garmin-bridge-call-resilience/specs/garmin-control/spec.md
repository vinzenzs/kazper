## ADDED Requirements

### Requirement: Bridge proxy calls survive client and gateway cancellation

Every backend `/garmin/*` handler that forwards to the garmin-bridge SHALL issue its outbound request on a context **decoupled from the inbound request context** — derived so that request-scoped values (trace/log correlation) are preserved while the inbound cancellation signal is dropped — bounded by an explicit per-operation timeout. A client or gateway that stops waiting (disconnect, gateway read timeout) SHALL NOT cancel an in-flight bridge operation; only the operation's own timeout or the bridge itself SHALL end it. Per-operation timeouts SHALL be right-sized by class rather than a single blanket value: a short budget for interactive/single-item calls (login, single read/write, hydration, rename/delete), and a longer budget for Garmin-paced or multi-item operations (calendar/library reads, blob export/download, plan/multisport fan-out). This guarantee applies to all bridge forwards; it changes no response shape on its own.

#### Scenario: A client disconnect does not abort the bridge operation

- **WHEN** an authenticated client triggers a proxied bridge operation and disconnects (or a gateway times out) before the bridge responds
- **THEN** the backend's outbound request to the bridge is NOT cancelled and the operation runs to completion server-side
- **AND** the (now-unread) bridge response is simply discarded

#### Scenario: A non-responsive bridge is bounded by the per-operation timeout

- **WHEN** the bridge accepts a forwarded request but never responds
- **THEN** the backend aborts the outbound call at the operation's own timeout and returns a gateway error to the caller, independent of any inbound-request lifetime

#### Scenario: Paced and fan-out operations get a longer budget than single-item calls

- **WHEN** a plan-scope or multisport push fans out several per-workout Garmin round-trips
- **THEN** the forward is bounded by the longer multi-item budget rather than the short interactive budget, so a normal fan-out completes without hitting the timeout

## MODIFIED Requirements

### Requirement: Backend proxy triggers a history backfill on the bridge

The system SHALL expose `POST /garmin/backfill` that forwards a `{from, to}` body to the garmin-bridge's `POST /sync/backfill` at `GARMIN_BRIDGE_URL`, returning the bridge's status code and body verbatim. Because the bridge now runs the backfill in the background and returns `202 Accepted` immediately with a `{run_id, from, to, days_total}` reference, the proxy forward is a short call: the backend SHALL forward that `202` (and its `run_id`) verbatim and SHALL NOT hold the client connection open for the duration of the paced replay. The endpoint SHALL add no fields and parse nothing beyond passing the body through. When `GARMIN_BRIDGE_URL` is unset, the endpoint SHALL return `503 garmin_disabled`. The endpoint SHALL require authentication. The forwarded call SHALL follow the cancellation-decoupling guarantee (an inbound disconnect never aborts the trigger before the bridge has accepted it).

#### Scenario: Backfill trigger forwards and returns the bridge's 202

- **WHEN** an authenticated client `POST`s `/garmin/backfill` with `{"from":"2026-03-01","to":"2026-04-30"}`
- **THEN** the backend forwards the body to the bridge's `POST /sync/backfill`
- **AND** returns the bridge's `202` and `{run_id, from, to, days_total}` body verbatim, without waiting for the replay to finish

#### Scenario: The caller polls sync-status for the outcome

- **WHEN** the client has received the `202` with a `run_id`
- **THEN** the per-day roll-up and terminal status are read via `GET /garmin/sync-status` (by that `run_id`), not from the trigger response

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and `POST /garmin/backfill` is called
- **THEN** the response is `503 garmin_disabled`

#### Scenario: Unauthenticated callers are rejected

- **WHEN** an unauthenticated client calls `POST /garmin/backfill`
- **THEN** the request is rejected by the auth middleware before any forward to the bridge
