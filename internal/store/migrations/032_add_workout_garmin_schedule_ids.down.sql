ALTER TABLE workouts
    DROP COLUMN IF EXISTS garmin_schedule_id,
    DROP COLUMN IF EXISTS garmin_workout_id;
