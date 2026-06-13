## Context

The garmin-bridge is a stateless FastAPI sidecar. A helm CronJob fires at 05:00 and POSTs `/sync` with no body; `do_sync` (`app.py:136`) resolves the date to "today" and calls `sync.sync_day(backend, fetch_day(api, date), date)` once. `fetch_day` (`garmin_client.py:110`) fans out per-capability Garmin calls, each wrapped in a `safe()` guard that logs a warning and returns `None` on failure. `sync_day` (`sync.py:33`) maps the raw day and posts to the date-keyed and `external_id`-keyed REST endpoints, all idempotent on re-run.

The miss is temporal, not structural: at 05:00 a day's own activities haven't happened, and Garmin's training-load / VO2max / race-predictor figures are recomputed later in the day after activities process. The same-day-only, never-revisit cadence therefore captures the overnight-ready recovery fields but systematically nulls activities and lagged fitness. The mappings already exist; the days just need re-pulling once the data is there.

There is already a `run_backfill` (`sync.py:147`) that walks a `[from, to]` range day-by-day, per-day tolerant and paced — the steady-state window is a smaller, default-on version of that same shape.

## Goals / Non-Goals

**Goals:**
- A dateless `POST /sync` re-pulls today + the previous `SYNC_LOOKBACK_DAYS` days (default 2) so late-arriving activities and lagged fitness land without operator action.
- Keep the CronJob a dumb trigger: it POSTs `/sync` with no body; the window lives in the service.
- Preserve targeted single-day sync (`POST /sync` with an explicit `date`) untouched, since the helm first-run backfill loop and any manual/MCP single-day calls depend on it.
- Per-day fault isolation within the window, mirroring existing per-capability and per-backfill-day tolerance.

**Non-Goals:**
- No change to `fetch_day`, `sync_day`, `map_*`, or any REST/MCP endpoint or Go code.
- No change to `/sync/backfill` (the explicit operator range-replay endpoint).
- No new scheduler — cadence stays in the helm CronJob.
- Not solving "sync the instant an activity completes" (webhooks/push); the rolling window is the bounded-polling answer.

## Decisions

**Window lives in `/sync`, not the cron shell.** A dateless `/sync` becomes correct for every caller (cron, MCP, manual curl), the behavior is unit-testable in `test_sync.py`, and correctness doesn't live in a brittle POSIX date-arithmetic shell loop. *Alternative considered:* keep `/sync` single-day and make the CronJob loop over the last N days (reuse the existing `backfillDays` shell loop as steady state). Rejected: pushes correctness into helm, untestable, and `date -d`/`date -r` portability already cost us a fallback branch in the template.

**Default `SYNC_LOOKBACK_DAYS = 2` (3-day window).** Covers next-day activity capture and the bulk of Garmin's load/VO2max compute lag, plus one day of slack for a missed cron run, at ~3× the daily fan-out — negligible for a single user. Configurable so it can widen without a code change. *Alternative considered:* 7 days — more outage-resilient but 2–3× the call volume for marginal benefit at steady state; left as a config bump if needed.

**Reuse the `run_backfill` per-day shape rather than invent a new loop.** Add a thin `run_window(backend, gc_fetch, api, dates)`-style helper (or parameterize the existing path) that iterates the resolved date list, calls `fetch_day` + `sync_day` per day inside a try/except that records a per-day result, and never aborts the window on a single day's failure. The dateless branch of `do_sync` builds the date list `[today - N, …, today]` (oldest-first) and delegates; the explicit-date branch keeps calling the single-day path. *Alternative considered:* literally call `run_backfill(today-N, today)`. Rejected only because `run_backfill` carries inter-day pacing/caps meant for long operator ranges; the steady-state window wants a lighter helper sharing the same per-day-tolerance contract. Pacing for a 3-day window is unnecessary but harmless if reused.

**Response shape: per-day results.** The dateless response returns a list/array of per-day outcomes (date + ok/failed + summary) so a partial window is observable. The explicit-date response keeps its existing single-day shape. HTTP status follows the existing convention: `200` when all synced days are ok, `207` when at least one day or capability was partial/failed.

**Date math uses the bridge's `sync_tz`.** "Today" and the window are computed in the configured sync timezone (the same `now(config.sync_tz)` / `_today` the cron path already uses), so the window aligns with Garmin's day boundaries rather than UTC.

## Risks / Trade-offs

- **[3× Garmin API volume on the daily run]** → Bounded (default 3 days), single user, and every call is already individually guarded/throttled-tolerant. Widening is a config knob, not a code change.
- **[A day permanently failing Garmin re-pulls every run]** → Acceptable: per-day tolerance means it never blocks other days; it's logged each run, and once outside the window it's dropped. No unbounded retry.
- **[Re-pulling overwrites a good prior snapshot with a later-degraded one]** → Low: date-keyed upserts are full-replace, but Garmin's recent-day data only gets *more* complete over the window, not less; recovery fields stay stable post-overnight. If a concern surfaces, a future refinement is field-level merge — explicitly out of scope here.
- **[Explicit-date callers accidentally lose lookback]** → Intended. The helm first-run `backfillDays` loop and manual single-day calls must stay exact-day; the spec makes this a normative distinction with its own scenario.

## Migration Plan

1. Ship the bridge image with `sync_lookback_days` (default 2) and the windowed dateless `/sync`.
2. Roll the helm chart: add `SYNC_LOOKBACK_DAYS` env to the bridge Deployment, keep the CronJob POSTing `/sync` with no body, set `cron.backfillDays: 0` (steady state). No backend redeploy, no migration.
3. One-off: `POST /sync/backfill` over 2026-06-10 → 2026-06-13 to fill the existing gap (06-11 activities + now-computed load/VO2max) immediately, rather than waiting for the window to crawl back.
4. **Rollback:** set `SYNC_LOOKBACK_DAYS = 0` (window collapses to today — original behavior) or redeploy the prior image. No data migration to reverse; all writes were idempotent upserts.
