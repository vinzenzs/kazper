## 1. Config

- [x] 1.1 Add `sync_lookback_days` (int, default `2`) to the bridge config in `apps/garmin-bridge/garmin_bridge/config.py`, sourced from `SYNC_LOOKBACK_DAYS`, validated as non-negative.
- [x] 1.2 Add a config test asserting the default is `2` and that `SYNC_LOOKBACK_DAYS` overrides it (and `0` is accepted).

## 2. Windowed sync logic

- [x] 2.1 In `apps/garmin-bridge/garmin_bridge/sync.py`, add a per-day-tolerant window helper (e.g. `run_window`) that takes an ordered list of dates and runs `fetch_day` + `sync_day` for each, capturing a per-day result and never aborting the window on a single day's failure (mirror `run_backfill`'s tolerance, without the operator-range pacing/cap).
- [x] 2.2 In `apps/garmin-bridge/garmin_bridge/app.py` `do_sync`: when no `date` is supplied, build the oldest-first date list `[today - sync_lookback_days, …, today]` using `config.sync_tz`, and delegate to the window helper; when an explicit `date` is supplied, keep the existing single-day path unchanged.
- [x] 2.3 Make the dateless response return a per-day result array; set HTTP status `200` when all synced days are ok and `207` when any day/capability is partial or failed (preserve the existing single-day response shape for explicit-date calls).

## 3. Tests

- [x] 3.1 `test_sync.py`: dateless `/sync` with `SYNC_LOOKBACK_DAYS=2` syncs exactly today + 2 prior days (3 `sync_day` invocations, correct dates, oldest-first).
- [x] 3.2 `test_sync.py`: explicit `date` syncs exactly that one day (no lookback).
- [x] 3.3 `test_sync.py`: one day in the window raising an error is recorded as failed and the remaining days still sync (window not aborted); response reports per-day outcome and `207`.
- [x] 3.4 `test_sync.py`: `SYNC_LOOKBACK_DAYS=0` collapses the dateless window to today only.

## 4. Deploy wiring

- [x] 4.1 Add `garminBridge.syncLookbackDays` (default `2`) to `deploy/helm/nutrition-api/values.yaml` with a comment explaining the rolling window and why same-day-only misses activities/lagged fitness.
- [x] 4.2 Wire `SYNC_LOOKBACK_DAYS` into the garmin-bridge Deployment env in `deploy/helm/nutrition-api/templates/garmin-bridge.yaml`.
- [x] 4.3 Update the CronJob notes/comments so it remains a dumb dateless `/sync` trigger for steady state (`backfillDays` stays the first-run-only knob); no per-day shell loop needed in steady state.

## 5. Docs

- [x] 5.1 Document the rolling window and the `SYNC_LOOKBACK_DAYS` knob in `apps/garmin-bridge/README.md` (dateless = window, explicit date = single day).

## 6. Verify

- [x] 6.1 Run the bridge test suite (`apps/garmin-bridge` pytest) and helm template lint/render; confirm green.
- [ ] 6.2 _(deferred — post-deploy operational step, needs the live bridge)_ Operational one-off: `POST /sync/backfill` over 2026-06-10 → 2026-06-13 to pull the missed 06-11 activities and now-computed load/VO2max; confirm `list_workouts` and the fitness snapshots for those dates fill in.
