-- Async backfill (per garmin-bridge-call-resilience). A backfill now runs in the
-- background and can no longer return its per-day roll-up in a synchronous
-- response body, so the outcome is recorded on the sync run and read back via
-- GET /garmin/sync-status. Two additive changes:
--   * `summary` (JSONB) carries the roll-up of a multi-day/long job (a backfill's
--     days_total/days_ok/days_failed plus per-day results).
--   * `partial` joins the status set to denote a run that completed with one or
--     more isolated per-day failures — distinct from `error` (a run-level
--     failure such as auth). Only `success` still counts toward freshness.
ALTER TABLE sync_runs
    DROP CONSTRAINT sync_runs_status_check,
    ADD CONSTRAINT sync_runs_status_check
        CHECK (status IN ('running', 'success', 'error', 'partial'));

ALTER TABLE sync_runs
    ADD COLUMN summary JSONB NULL;
