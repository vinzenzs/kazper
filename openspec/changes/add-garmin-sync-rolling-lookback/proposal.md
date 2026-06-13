## Why

The bridge's daily sync runs once at 05:00 and POSTs `/sync` for **today only**, then never revisits that day. But two of the most coaching-critical signal classes aren't available at 05:00: a day's **completed activities** happen *after* the sync runs (and the next day's run only syncs the next day, so a day's own workouts are never captured), and Garmin's **training-load / VO2max / race-predictor** metrics are recomputed *later in the day* once activities process — so they're stale or absent at 05:00 and `safe()` silently nulls them. The result is fitness snapshots with only the overnight-ready fields (endurance score, fitness age) and activities that never land in the `workouts` table. Date-keyed upserts and `external_id` workout dedup already make re-syncing a day safe, so the fix is to re-pull recent days, not to add new write paths.

## What Changes

- `POST /sync` **with no `date`** syncs a **rolling window** of recent days — today and the previous `SYNC_LOOKBACK_DAYS` days (default `2` → a 3-day window) — instead of only today. Each day in the window reuses the existing `fetch_day` + `sync_day` path, so all current mappings, per-capability failure tolerance, and idempotent upserts apply unchanged per day.
- `POST /sync` **with an explicit `date`** continues to sync exactly that one day (no lookback) — preserving the targeted/manual and CronJob-backfill behaviors.
- New bridge config `SYNC_LOOKBACK_DAYS` (default `2`), surfaced through `config.py`, the helm values, and the bridge env wiring. The CronJob stays a dumb trigger — it keeps POSTing `/sync` with no body; the window lives in the service.
- Each day in the window is synced independently and tolerant of a single bad day (one day's failure does not abort the rest of the window), mirroring the existing per-capability and per-backfill-day tolerance. The `/sync` response reports a per-day result for the window.
- A one-off operational `POST /sync/backfill` over the recent gap (2026-06-10 → 2026-06-13) to immediately pull in the missed 06-11 activities and any now-computed load/VO2max for those days. The backfill endpoint itself is unchanged.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `garmin-bridge`: the "Headless daily sync" requirement changes so a dateless `POST /sync` syncs a bounded rolling window of recent days (today + `SYNC_LOOKBACK_DAYS`) rather than a single day, while an explicit `date` still syncs just that day; the window is per-day tolerant and idempotent.

## Impact

- **Bridge code**: `apps/garmin-bridge/garmin_bridge/config.py` (new `sync_lookback_days`), `app.py` (`do_sync` expands the dateless path to a window), `sync.py` (window iteration reusing `sync_day`; likely a small `run_window` helper that mirrors `run_backfill`'s per-day tolerance).
- **Tests**: `apps/garmin-bridge/tests/test_sync.py` (dateless `/sync` hits N days; explicit `date` hits exactly one; one failing day doesn't sink the window; lookback honored from config).
- **Deploy**: `deploy/helm/nutrition-api/values.yaml` + the garmin-bridge Deployment env in `templates/garmin-bridge.yaml` gain `SYNC_LOOKBACK_DAYS`; the CronJob shell is simplified/clarified (no per-day loop needed for steady state). `backfillDays` first-run knob remains for the initial import.
- **Docs**: `apps/garmin-bridge/README.md` documents the rolling window and the config knob.
- **No backend (Go) changes, no migrations, no REST/MCP surface changes** — the nutrition API endpoints and their upsert semantics are untouched.
- **Garmin API call volume**: ~3× the per-day fan-out on the daily run (bounded, paced by the existing per-call guards); negligible for a single user.
