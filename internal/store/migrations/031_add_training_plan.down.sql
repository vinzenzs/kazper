DROP INDEX IF EXISTS workouts_plan_slot_id_key;
ALTER TABLE workouts
    DROP COLUMN IF EXISTS plan_slot_id,
    DROP COLUMN IF EXISTS template_id;

DROP TABLE IF EXISTS plan_slots;
DROP TABLE IF EXISTS plan_weeks;
DROP TABLE IF EXISTS training_plans;
