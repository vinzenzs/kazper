## Context

The backend is a thin proxy in front of the garmin-bridge: `internal/garmincontrol` handlers forward each `/garmin/*` operation to the bridge at `GARMIN_BRIDGE_URL`. Today every handler builds its outbound request with `http.NewRequestWithContext(c.Request.Context(), …)`:

- `forward()` (handlers.go:150) and the library/scheduling handlers use a shared `h.client` with `forwardTimeout = 30 * time.Second` (handlers.go:52).
- `backfill()` (library.go:48) uses a dedicated `&http.Client{Timeout: 30 * time.Minute}` but is still bound to `c.Request.Context()`.
- `schedule/plan` and `schedule/multisport` (scheduling.go) fan out **many** per-workout Garmin round-trips, each through the 30 s client and the request context.

Two failure modes result, both observed in the field:
1. **Cancellation coupling.** An ingress/gateway ahead of the backend enforces its own (shorter) read timeout. When it gives up on a slow bridge round-trip and closes the client connection, Go cancels `c.Request.Context()`, which cancels the outbound request — tearing down a bridge operation that was completing correctly. A running backfill imports day-by-day the whole time; only the held-open HTTP request "fails."
2. **Undersized timeouts.** A paced range backfill and a multi-workout plan push legitimately run longer than 30 s (or than the gateway budget), so even absent an abort the caller reads a spurious failure and retries — re-triggering paced Garmin work and deepening rate-limiting.

The bridge already models long-running work: it opens a `sync_runs` row (`running` → `success`/`error`, carrying `window_from`/`window_to`) via `open_sync_run`/`close_sync_run` (garmin_bridge/backend.py), and the backend exposes it at `GET /garmin/sync-status` (`internal/garminsyncstatus`).

## Goals / Non-Goals

**Goals:**
- No bridge proxy operation is aborted or reported as failed merely because a gateway/client stopped waiting.
- Long, fire-and-forget writes (backfill; plan/multisport scheduling) return promptly and are observable to completion via polling.
- Timeouts reflect each operation's real duration class rather than one blanket value.
- The daily `POST /sync` path, pacing, range cap, and idempotency are untouched.

**Non-Goals:**
- Converting synchronous **reads** (calendar, get-workout, export, download, activity-gear) to async — a reader needs its data back. These get decoupling + a right-sized timeout only.
- A general job queue/orchestration system. We reuse the existing `sync_runs` model; a plan-push job uses the same run abstraction, not new infrastructure.
- Ingress-timeout tuning as a code deliverable (documented for deployment; the code fix must stand on its own regardless of the gateway value).

## Decisions

### D1 — Decouple every outbound bridge request from the inbound request context
Introduce a single forwarding path that derives the outbound context from `context.WithoutCancel(c.Request.Context())` plus an explicit `context.WithTimeout` for the operation's budget. The bridge call then survives the client/gateway going away; only the operation's own timeout (or the bridge) can end it.

- **Why not keep `c.Request.Context()`?** It conflates "client is still waiting" with "the operation should continue." For a proxied side-effecting write, those are different; we want the write to finish.
- **Alternative considered — detached `context.Background()` with timeout:** equivalent for cancellation, but `WithoutCancel` preserves request-scoped values (trace/log correlation) while dropping only the cancellation signal. Preferred.
- **Reads:** the same decoupling applies; a cancelled reader simply no longer receives the (still-produced) body, which is harmless.

### D2 — Right-size timeouts by operation class
Replace the single 30 s constant with a small set of budgets: a short interactive budget (login, single-item read/write, hydration, rename/delete), a medium budget (calendar/library reads, export/download of one blob), and a long budget for the paced/fan-out jobs. Keep them as named constants in `garmincontrol` so they are visible and tunable.

### D3 — Backfill becomes `202 Accepted` + poll (bridge-side async)
The paced range replay is the one operation that structurally can't fit any sane gateway budget, and the `sync_runs` model already fits it. The bridge validates the range, opens the sync run, launches the paced replay as a background task (FastAPI `BackgroundTasks`/`asyncio.create_task`), and returns `202 {run_id, from, to, days_total}`. The background task records the roll-up and closes the run `success` (all days ok), `partial` (≥1 day failed), or `error` (the run itself failed, e.g. auth). The backend proxy forwards the `202` verbatim — its call is now fast, so D1/D2 make its coupling moot.

- **Why not also make plan/multisport `202` in this change?** Their per-workout fan-out lives in the backend loop (scheduling.go) and has no run model to poll; adding one is a larger, separable surface. A plan *week* is a handful of workouts — under D1/D2 (detached context + a medium/long budget) it completes server-side and fits a reasonable gateway budget. So plan/multisport get the universal resilience fix now; converting them to `202`-async (a backend-side background job + its own pollable run) is a **deferred follow-up** for if plan spans grow large.
- **Alternative considered — backend-side async for backfill too:** the backend could background its own forward and expose a job. Rejected: it duplicates the run bookkeeping the bridge already owns and splits the source of truth. Bridge-side async keeps the backend a thin proxy.

### D4 — Surface the outcome on the sync run for polling
The roll-up that used to return inline (`days_total`/`days_ok`/`days_failed`, and per-workout results for plan pushes) is written to the run and exposed by `GET /garmin/sync-status`. Prefer storing a compact JSON summary on the run row over adding many typed columns, minimizing migration surface. A partial backfill closes the run in a terminal state that still conveys "some days failed" (either `error` with the roll-up, or an explicit partial status — resolved in specs).

## Risks / Trade-offs

- **Async hides failures behind a poll** → the terminal state on the run must be unambiguous (success vs partial vs error) and carry enough detail to act on; `GET /garmin/sync-status` is the single source of truth and its shape is specified, not left implicit.
- **Concurrent runs mask each other** → a backgrounded backfill and a daily cron `/sync` both open runs; `sync-status` returning only "the latest" could hide the backfill. Mitigation: return the relevant run by id (the `202` hands back `run_id`) and/or distinguish run kind; specified in `garmin-sync-status`.
- **Background task lifecycle** → a bridge restart mid-backfill leaves a run stuck `running`. Mitigation: the run is idempotent to re-trigger (date-keyed upserts + `external_id` dedup), and a stale-`running` run is re-runnable; note operationally, no new reaper in this change.
- **Detached context could outlive intent** → an operator who "cancels" by disconnecting no longer stops the work. Accepted: for these idempotent, side-effecting writes, completing is the desired behavior; the timeout still bounds runaway calls.
- **BREAKING response shape** → callers expecting the synchronous `200`/`207` body must move to polling. Mitigation: update MCP tool descriptions, `docs/`, and helper messaging in the same change; the `202` body names the run to poll.

## Migration Plan

- Ship backend + bridge together (the `202` contract spans both); the bridge change is backward-tolerant in that the backend forwards whatever status the bridge returns.
- No data migration unless a summary column is added to `sync_runs`; prefer a JSON summary to keep it to at most one additive, nullable column with a standard `NNN_*.up/down.sql` pair.
- Rollback: revert the tag; the prior synchronous behavior returns. In-flight background jobs from the new version simply complete or are re-triggerable.

## Open Questions

- Partial-backfill terminal state: reuse `error` with a roll-up, or introduce an explicit `partial` status on `sync_runs` (CHECK-constraint change)? — resolve in the `garmin-sync-status` spec.
- Do plan/multisport pushes warrant their own run `kind`, or is a shared run row with a summary sufficient to disambiguate from daily syncs when polling?
- Should `sync-status` accept a `run_id` query to fetch a specific run, versus always returning the most recent (needed when backfill + daily sync overlap)?
