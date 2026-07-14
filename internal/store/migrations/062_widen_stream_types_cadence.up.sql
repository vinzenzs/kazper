-- Cadence streams (add-cadence-quadrant-analysis). Quadrant analysis needs a
-- cadence (rpm) series alongside power; migration 056's workout_streams CHECK
-- admitted only power/speed/heart_rate. Widen it to accept 'cadence' — storage
-- semantics are unchanged (another 1 Hz REAL[] series, replace-on-repost,
-- cascade). Cadence feeds no best-effort, no execution metric, no energy total.
ALTER TABLE workout_streams DROP CONSTRAINT workout_streams_stream_type_check;

ALTER TABLE workout_streams
    ADD CONSTRAINT workout_streams_stream_type_check
    CHECK (stream_type IN ('power', 'speed', 'heart_rate', 'cadence'));
