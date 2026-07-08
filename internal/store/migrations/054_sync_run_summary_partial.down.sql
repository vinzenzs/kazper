-- Reverse 054. Any `partial` runs must be folded back into the 3-value set
-- before the narrower CHECK is restored (treated as errors — they are non-success
-- outcomes).
UPDATE sync_runs SET status = 'error' WHERE status = 'partial';

ALTER TABLE sync_runs
    DROP CONSTRAINT sync_runs_status_check,
    ADD CONSTRAINT sync_runs_status_check
        CHECK (status IN ('running', 'success', 'error'));

ALTER TABLE sync_runs
    DROP COLUMN summary;
