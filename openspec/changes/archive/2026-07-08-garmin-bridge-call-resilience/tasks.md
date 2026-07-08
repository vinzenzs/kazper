## 1. Backend: decouple all bridge forwards from the inbound request context

- [x] 1.1 Add a forwarding-context helper in `internal/garmincontrol` that derives an outbound context via `context.WithoutCancel(c.Request.Context())` plus a per-operation `context.WithTimeout`, so request-scoped values survive but the inbound cancellation signal is dropped.
- [x] 1.2 Replace the timeout budgets: define named per-class constants (short interactive/single-item; longer paced/fan-out) to supersede the blanket `forwardTimeout = 30s`.
- [x] 1.3 Route every outbound bridge request through the helper — `forward()` (handlers.go:150), the library handlers (library.go:96/226), and the scheduling handlers (scheduling.go:100/147/228/287/355/400/553) — replacing each `http.NewRequestWithContext(c.Request.Context(), …)` and picking the right budget per handler.
- [x] 1.4 Test: a simulated client disconnect mid-forward does not cancel the outbound bridge call (the op completes; the response is discarded), and a non-responsive bridge is cut off at the operation budget.

## 2. Bridge: make backfill non-blocking (`202` + background replay)

- [x] 2.1 In `apps/garmin-bridge/garmin_bridge/app.py`, split `POST /sync/backfill`: validate + cap the range synchronously (keep `400 range_too_large`, open no run, write nothing on reject); on accept, open the sync run and schedule the paced replay as a background task, returning `202 {run_id, from, to, days_total}` immediately.
- [x] 2.2 Move the day-by-day replay loop into the background task (unchanged per-day `sync_day` path, oldest-first, `BACKFILL_DAY_DELAY_SECONDS` pacing, per-day isolation).
- [x] 2.3 On completion, record the roll-up (`days_total`/`days_ok`/`days_failed` + per-day results) via `close_sync_run`, closing `success` (all ok) / `partial` (≥1 day failed) / `error` (run-level failure).
- [x] 2.4 Update bridge tests (`tests/test_backfill.py`): assert `202` + `run_id` returned before the replay finishes, background replay syncs each day, and terminal run status/summary are recorded (all mocked; no live Garmin).

## 3. Backend: `sync_runs` gains `partial` status and a `summary` roll-up

- [x] 3.1 Add a migration (054) that adds a nullable `summary jsonb` column and widens the `status` CHECK to `running|success|error|partial`.
- [x] 3.2 Extend `internal/garminsyncstatus` types/repo to carry `summary` and accept `partial` on close; ensure `close_sync_run`'s PATCH accepts the roll-up + `partial`.
- [x] 3.3 Update `GET /garmin/sync-status`: accept optional `?run_id=` (return that run as `latest`, `404` on unknown id); include each run's `summary` when present; keep only `status=success` counting toward `last_successful_at`/`is_stale` (a `partial`/`error` run is not a success).
- [x] 3.4 Tests: fetch-by-`run_id` (including running→partial transition and `404`), `partial` excluded from `last_successful_at`/`is_stale`, `summary` surfaced.

## 4. Backend proxy: forward the bridge's `202` for backfill

- [x] 4.1 Update `backfill()` (library.go) to forward `{from,to}` and return the bridge's `202` + `{run_id,…}` verbatim through the decoupled helper; drop the bespoke 30-min client / `c.Request.Context()` coupling (superseded by task 1). Keep `503 garmin_disabled` when the bridge URL is unset and the auth requirement.
- [x] 4.2 Test the proxy: `202` + `run_id` passthrough, disabled/unauth paths unchanged.

## 5. Clients, docs, and surface sync

- [x] 5.1 Update the MCP `garmin_backfill` tool description to the trigger-then-poll flow (returns a `run_id`; read the outcome via `garmin_sync_status`).
- [x] 5.2 Run `task swag` to regenerate `docs/` for the changed `/garmin/backfill` and `/garmin/sync-status` shapes (`json.RawMessage` summary annotated `swaggertype:"object"`).
- [x] 5.3 Checked `RUN_LOCAL.md`/README and the `task garmin:*` helpers — no existing doc or task target describes the garmin backfill flow (its user-facing docs are the swagger + MCP tool descriptions, both updated), so no change needed.

## 6. Verify

- [x] 6.1 Affected Go packages (`garminsyncstatus`, `garmincontrol`, `agenttools`, `httpserver`) and the bridge suite (148 tests, Python 3.13 venv) green.
- [x] 6.2 `go vet ./...` clean; `go build ./...` clean.
- [x] 6.3 Async backfill behavior verified via the HTTP-level integration test (real FastAPI route + background task): `202` returned immediately, background replay records the run's terminal `status`/`summary`; sync-status `?run_id=` transitions `running → success/partial`. (Automated equivalent of the manual live-stack run.)
