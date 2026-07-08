## Why

The backend proxies every Garmin operation to the garmin-bridge, and each proxy handler in `internal/garmincontrol` builds its outbound request with `http.NewRequestWithContext(c.Request.Context(), …)` behind a shared 30-second client (backfill excepted, at 30 minutes). Two consequences fall out of that shape:

1. **A gateway/client timeout aborts in-flight bridge work.** When an ingress in front of the backend gives up on a slow Garmin round-trip and drops the client, `c.Request.Context()` is cancelled and the backend tears down the bridge call mid-operation — even though the bridge was completing the work correctly. This is what made a running backfill look like a failure: the import was importing day-by-day the whole time; only the held-open HTTP request timed out.
2. **The slow calls exceed the timeout at all.** A paced range backfill runs for minutes, and `schedule/plan` / `schedule/multisport` fan out many per-workout Garmin round-trips through the 30 s client — both legitimately outlast the client/gateway budget, so the caller reads a spurious failure and retries, kicking off more work and deepening Garmin rate-limiting.

The bridge operations are correct; the synchronous, cancellation-coupled request shape is the defect — and it is not unique to backfill.

## What Changes

- **Every** `garmincontrol` bridge-proxy call decouples its outbound request from the inbound request context — forwarding on a context derived from `context.WithoutCancel(...)` with an explicit per-operation timeout — so a gateway/client disconnect can no longer cancel an in-flight bridge operation. The operation runs to completion server-side regardless of whether the caller is still waiting.
- Per-operation **timeouts are right-sized** by class instead of one blanket 30 s: fast single-item reads/writes keep a short budget; multi-item and Garmin-paced operations get a budget that reflects their real duration.
- The structurally-longest, fire-and-forget write job — the paced range **backfill** — becomes non-blocking (**`202 Accepted` + poll**): bridge `POST /sync/backfill` validates the range, opens a sync run, runs the paced day-by-day replay in the background, and returns `202 {run_id, from, to, days_total}` immediately. **BREAKING**: it no longer returns the synchronous `200`/`207` per-day summary.
- The backfill **outcome that used to come back inline is surfaced for polling**: its roll-up (`days_total`/`days_ok`/`days_failed`) and a terminal status (`success` / `partial` / `error`) are recorded on the sync run and read via `GET /garmin/sync-status` (by `run_id`).
- All other proxied operations — the plan/multisport fan-outs, reads (calendar, library, export/download, gear), and single-item writes — keep their synchronous response shape but now complete server-side under the decoupling + a right-sized timeout, so a gateway/client timeout can neither abort them nor mislead the caller. (Converting the plan/multisport fan-out to `202`-async is a noted follow-up should plan spans grow past a sane gateway budget.)
- Callers (coach/MCP, operator) shift from "await the response" to "trigger, then poll" for backfill; other calls are unchanged in shape but no longer abort under gateway pressure.

## Capabilities

### New Capabilities
<!-- none — this hardens existing behavior -->

### Modified Capabilities
- `garmin-control`: the backend Garmin-proxy requirements gain a cancellation-decoupling + per-operation-timeout guarantee for **all** bridge forwards; the backfill proxy additionally forwards/returns the bridge's `202` and no longer holds the client for the full operation.
- `garmin-bridge`: the "Bounded, paced, idempotent history backfill" requirement changes from a synchronous `200`/`207` replay to a background replay returning `202` immediately and recording its roll-up + terminal status on the sync run.
- `garmin-sync-status`: the sync run / `GET /garmin/sync-status` surface the backfill (and long-job) roll-up and range so a polling caller reads the outcome that used to arrive in the synchronous body.

## Impact

- **Code**: `internal/garmincontrol/{handlers.go,library.go,scheduling.go}` (detached-context forwarding helper + per-op timeouts across all forwards; backfill → `202`); `apps/garmin-bridge/garmin_bridge/app.py` (backfill handler → background task + `202`) and `sync.py`/`backend.py` (roll-up on run close); `internal/garminsyncstatus/` (carry the roll-up + `partial` status + `run_id` read, with a migration).
- **API surface**: `POST /garmin/backfill` and bridge `POST /sync/backfill` now respond `202`; `GET /garmin/sync-status` gains a roll-up summary, a `partial` status, and an optional `run_id` selector. `task swag` re-run required; `docs/`, `RUN_LOCAL.md`, and the `task garmin:*` helper messaging updated to trigger-then-poll.
- **Clients**: the MCP `garmin_backfill` tool description and coach guidance shift to the poll-sync-status pattern; it no longer returns per-day results inline.
- **Ops**: reduces retry-storms against Garmin (spurious client failures no longer trigger re-triggers); no change to pacing, the range cap, idempotency (date-keyed upserts + `external_id` dedup), or the daily `POST /sync` path.
- **Non-goals**: not converting synchronous reads (calendar, get-workout, export/download, activity-gear) to async — those rely on decoupling + a right-sized timeout within the gateway budget; ingress-timeout tuning for the `/garmin/*` path is noted for deployment but not a code deliverable here.
