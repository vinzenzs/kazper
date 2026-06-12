-- Track what has been pushed to the Garmin watch (per add-garmin-scheduling).
-- garmin_workout_id is the structured workout created in the Garmin library;
-- garmin_schedule_id is the calendar entry scheduling it. Both are opaque Garmin
-- ids — stored and echoed, never parsed — so unschedule and re-push are clean
-- and a session is never double-created in the library.
ALTER TABLE workouts
    ADD COLUMN garmin_workout_id  TEXT NULL,
    ADD COLUMN garmin_schedule_id TEXT NULL;
