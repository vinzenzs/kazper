-- Daily recovery snapshot from a wellness source (Garmin today). One row per
-- calendar date; the importer "POSTs every day it sees" and re-pushes upsert in
-- place via the date primary key. Every metric is nullable — NULL means "the
-- device did not report it that day," a meaningful state.

CREATE TABLE recovery_metrics (
    date                 DATE PRIMARY KEY,
    sleep_seconds        INTEGER     CHECK (sleep_seconds IS NULL OR sleep_seconds > 0),
    sleep_score          INTEGER     CHECK (sleep_score IS NULL OR (sleep_score BETWEEN 0 AND 100)),
    hrv_ms               NUMERIC(6,1) CHECK (hrv_ms IS NULL OR hrv_ms > 0),
    resting_hr           INTEGER     CHECK (resting_hr IS NULL OR resting_hr > 0),
    stress_avg           INTEGER     CHECK (stress_avg IS NULL OR (stress_avg BETWEEN 0 AND 100)),
    body_battery_charged INTEGER     CHECK (body_battery_charged IS NULL OR (body_battery_charged BETWEEN 0 AND 100)),
    body_battery_drained INTEGER     CHECK (body_battery_drained IS NULL OR (body_battery_drained BETWEEN 0 AND 100)),
    training_readiness   INTEGER     CHECK (training_readiness IS NULL OR (training_readiness BETWEEN 0 AND 100)),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
