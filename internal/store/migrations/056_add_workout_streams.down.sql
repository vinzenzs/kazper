ALTER TABLE workouts DROP COLUMN IF EXISTS decoupling_pct;
ALTER TABLE workouts DROP COLUMN IF EXISTS efficiency_factor;
ALTER TABLE workouts DROP COLUMN IF EXISTS variability_index;
DROP TABLE IF EXISTS workout_streams;
