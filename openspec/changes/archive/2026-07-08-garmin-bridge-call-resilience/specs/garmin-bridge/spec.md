## MODIFIED Requirements

### Requirement: Bounded, paced, idempotent history backfill over a date range

The bridge SHALL expose `POST /sync/backfill` accepting an inclusive `from` and `to` date (`YYYY-MM-DD`). The range SHALL be capped: when the requested span exceeds `BACKFILL_MAX_DAYS`, the bridge SHALL reject the request synchronously with `400 range_too_large` (including the cap), opening no sync run and writing nothing. For an accepted range the bridge SHALL open a sync run covering `[from, to]`, launch the replay as a **background task**, and return `202 Accepted` immediately with `{run_id, from, to, days_total}` — it SHALL NOT block until the range completes and SHALL NOT return a synchronous per-day body. The background replay SHALL process every date in `[from, to]` oldest-first through the same token-load + day-fetch + `sync_day` path used by `POST /sync` (so any per-activity or per-day enrichment is picked up with no backfill-specific mapping), pausing `BACKFILL_DAY_DELAY_SECONDS` between consecutive days to pace Garmin calls. Each day SHALL be isolated: a failing day SHALL be recorded and the range SHALL continue. On completion the background task SHALL record the per-day results and roll-up (`days_total`, `days_ok`, `days_failed`) on the sync run and close it: `success` when every day succeeded, `partial` when the range completed with one or more failed days, and `error` when the run itself failed (e.g. auth/token). Re-running the same range SHALL be idempotent via the existing date-keyed upserts and `external_id` activity dedup — no new dedup mechanism.

#### Scenario: An accepted range returns 202 immediately and runs in the background

- **WHEN** `POST /sync/backfill` is called with `{"from":"2026-03-01","to":"2026-03-05"}` and a valid stored token
- **THEN** the bridge opens a sync run for the range and returns `202` with `{run_id, from, to, days_total}` without waiting for the replay to finish
- **AND** the paced day-by-day replay proceeds in the background, syncing each date `2026-03-01`…`2026-03-05` inclusive through the same per-day mapping as `POST /sync`
- **AND** whatever enrichment the per-day path produces is written for each day with no backfill-specific field handling

#### Scenario: The completed run records the roll-up for polling

- **WHEN** a background backfill finishes all of its days successfully
- **THEN** the run is closed `success` with a recorded roll-up of `days_total`, `days_ok`, and `days_failed`
- **AND** the outcome is readable via `GET /garmin/sync-status` (by `run_id`) rather than from the trigger response

#### Scenario: One bad day does not abort the range and closes the run partial

- **WHEN** a backfill spans several days and the Garmin fetch or a backend write fails for exactly one of them
- **THEN** that day's entry records its error (`{date, ok:false, error}`) and the remaining days still sync
- **AND** the run closes with status `partial`, a roll-up of `days_failed` ≥ 1, and re-issuing the backfill for the failed date alone re-attempts only that day

#### Scenario: An over-cap range is rejected before any work

- **WHEN** `POST /sync/backfill` is called with a span exceeding `BACKFILL_MAX_DAYS`
- **THEN** the response is `400 range_too_large` (including the cap), no sync run is opened, and nothing is written

#### Scenario: Pacing inserts a delay between days

- **WHEN** a multi-day backfill runs with `BACKFILL_DAY_DELAY_SECONDS` set to a positive value
- **THEN** the background replay pauses that many seconds between consecutive days
