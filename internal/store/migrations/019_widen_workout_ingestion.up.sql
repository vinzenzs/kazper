-- Widen the workouts ingestion shape with source-agnostic fields the Garmin
-- importer (and any other writer) measures but the table could not carry:
--   distance_m     — session distance in metres (> 0)
--   avg_power_w    — average power in watts (> 0); stored for work/kJ arithmetic
--   temperature_c  — ambient temperature in °C, [-40, 60]; heat context for fuelling
--   sweat_loss_ml  — estimated sweat loss in ml (> 0); personalises fluid targets
--   session_group  — free-text key linking the legs of a brick/multisport session
--
-- All nullable, no back-fill — existing rows read NULL ("not measured" / "not
-- grouped" is a meaningful state, mirroring the rpe/gi precedent). CHECK
-- constraints give defence-in-depth alongside handler-level validation.

ALTER TABLE workouts
    ADD COLUMN distance_m NUMERIC(10, 1)
        CHECK (distance_m IS NULL OR distance_m > 0);

ALTER TABLE workouts
    ADD COLUMN avg_power_w INTEGER
        CHECK (avg_power_w IS NULL OR avg_power_w > 0);

ALTER TABLE workouts
    ADD COLUMN temperature_c NUMERIC(4, 1)
        CHECK (temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60));

ALTER TABLE workouts
    ADD COLUMN sweat_loss_ml NUMERIC(10, 1)
        CHECK (sweat_loss_ml IS NULL OR sweat_loss_ml > 0);

ALTER TABLE workouts
    ADD COLUMN session_group TEXT;

-- Partial (non-unique) index so fetching the legs of one session by group key
-- stays cheap without bloating the index with the common NULL case.
CREATE INDEX workouts_session_group_idx
    ON workouts (session_group)
    WHERE session_group IS NOT NULL;
