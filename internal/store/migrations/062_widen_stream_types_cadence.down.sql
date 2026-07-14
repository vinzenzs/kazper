-- Drop cadence rows, then narrow the CHECK back to the pre-cadence set.
DELETE FROM workout_streams WHERE stream_type = 'cadence';

ALTER TABLE workout_streams DROP CONSTRAINT workout_streams_stream_type_check;

ALTER TABLE workout_streams
    ADD CONSTRAINT workout_streams_stream_type_check
    CHECK (stream_type IN ('power', 'speed', 'heart_rate'));
