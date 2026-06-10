DROP INDEX IF EXISTS workouts_session_group_idx;
ALTER TABLE workouts DROP COLUMN IF EXISTS session_group;
ALTER TABLE workouts DROP COLUMN IF EXISTS sweat_loss_ml;
ALTER TABLE workouts DROP COLUMN IF EXISTS temperature_c;
ALTER TABLE workouts DROP COLUMN IF EXISTS avg_power_w;
ALTER TABLE workouts DROP COLUMN IF EXISTS distance_m;
