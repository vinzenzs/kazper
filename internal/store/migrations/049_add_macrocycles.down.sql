-- Reverse 049_add_macrocycles. Drop the phase columns (and index) first so the
-- macrocycles FK has no dependents, then drop the table. Non-destructive to
-- every other training_phases field — phase rows survive, having only lost the
-- season link and progression targets.
DROP INDEX IF EXISTS training_phases_macrocycle_id_idx;
ALTER TABLE training_phases
    DROP COLUMN IF EXISTS target_weekly_hours,
    DROP COLUMN IF EXISTS target_weekly_tss,
    DROP COLUMN IF EXISTS macrocycle_ordinal,
    DROP COLUMN IF EXISTS macrocycle_id;

DROP INDEX IF EXISTS macrocycles_date_range_idx;
DROP TABLE IF EXISTS macrocycles;
