-- Restore the pre-tier state: drop tiered rows, narrow the unique key back to
-- (workout_id, metric, duration_s), and drop the column.
DELETE FROM workout_best_efforts WHERE kj_tier > 0;

DROP INDEX workout_best_efforts_uidx;

CREATE UNIQUE INDEX workout_best_efforts_uidx
    ON workout_best_efforts (workout_id, metric, duration_s);

ALTER TABLE workout_best_efforts DROP COLUMN kj_tier;
