DROP INDEX IF EXISTS workouts_status_idx;
ALTER TABLE workouts DROP COLUMN IF EXISTS status;
