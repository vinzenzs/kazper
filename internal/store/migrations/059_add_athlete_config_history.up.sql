-- Append-only history of the athlete_config singleton (add-threshold-history).
-- The singleton stays the authoritative "current" read; this records the full
-- physiology state each time a PUT changes it, dated by effective_from (a DATE:
-- thresholds are per-day quantities and the PUT date is the only honest stamp).
-- Same-day changes collapse to one row (PK on effective_from); the service keeps
-- the history canonical (no two consecutive rows physiology-identical).
CREATE TABLE athlete_config_history (
    effective_from DATE PRIMARY KEY,

    ftp_watts                        INTEGER NULL CHECK (ftp_watts > 0),
    threshold_hr                     INTEGER NULL CHECK (threshold_hr > 0),
    lactate_threshold_hr             INTEGER NULL CHECK (lactate_threshold_hr > 0),
    max_hr                           INTEGER NULL CHECK (max_hr > 0),
    threshold_pace_sec_per_km        NUMERIC NULL CHECK (threshold_pace_sec_per_km > 0),
    threshold_swim_pace_sec_per_100m NUMERIC NULL CHECK (threshold_swim_pace_sec_per_100m > 0),
    hr_zone_1_max                    INTEGER NULL CHECK (hr_zone_1_max > 0),
    hr_zone_2_max                    INTEGER NULL CHECK (hr_zone_2_max > 0),
    hr_zone_3_max                    INTEGER NULL CHECK (hr_zone_3_max > 0),
    hr_zone_4_max                    INTEGER NULL CHECK (hr_zone_4_max > 0),
    hr_zone_5_max                    INTEGER NULL CHECK (hr_zone_5_max > 0),
    power_zone_1_max                 INTEGER NULL CHECK (power_zone_1_max > 0),
    power_zone_2_max                 INTEGER NULL CHECK (power_zone_2_max > 0),
    power_zone_3_max                 INTEGER NULL CHECK (power_zone_3_max > 0),
    power_zone_4_max                 INTEGER NULL CHECK (power_zone_4_max > 0),
    power_zone_5_max                 INTEGER NULL CHECK (power_zone_5_max > 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed one baseline snapshot from the existing singleton (0 or 1 rows) at the
-- epoch sentinel: "this is the oldest state we know, assume it for everything
-- earlier" — makes ConfigAsOf total for any date whenever a config exists. The
-- row's created_at records when tracking actually began.
INSERT INTO athlete_config_history (
    effective_from,
    ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
    threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
    hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
    power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max
)
SELECT
    DATE '1970-01-01',
    ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
    threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
    hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
    power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max
FROM athlete_config;
