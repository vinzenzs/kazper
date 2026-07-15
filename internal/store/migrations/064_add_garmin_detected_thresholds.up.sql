-- Split the Garmin threshold write path off the deliberate athlete_config
-- singleton (separate-garmin-threshold-detection). Two additive changes:
--
-- 1. garmin_detected_thresholds — a latest-detection singleton (fixed PK) the
--    bridge PUTs each sync. Holds only the physiology Garmin can map (FTP,
--    lactate-threshold HR, max HR, running threshold pace, HR-zone and Coggan
--    power-zone maxima) plus detected_at. Advisory evidence: never read or
--    mutated by an athlete_config write, never snapshotted into history. It is
--    latest-only (re-derived by the next sync) and export-excluded.
CREATE TABLE garmin_detected_thresholds (
    id UUID PRIMARY KEY,

    ftp_watts                 INTEGER NULL CHECK (ftp_watts > 0),
    lactate_threshold_hr      INTEGER NULL CHECK (lactate_threshold_hr > 0),
    max_hr                    INTEGER NULL CHECK (max_hr > 0),
    threshold_pace_sec_per_km NUMERIC NULL CHECK (threshold_pace_sec_per_km > 0),

    hr_zone_1_max INTEGER NULL CHECK (hr_zone_1_max > 0),
    hr_zone_2_max INTEGER NULL CHECK (hr_zone_2_max > 0),
    hr_zone_3_max INTEGER NULL CHECK (hr_zone_3_max > 0),
    hr_zone_4_max INTEGER NULL CHECK (hr_zone_4_max > 0),
    hr_zone_5_max INTEGER NULL CHECK (hr_zone_5_max > 0),

    power_zone_1_max INTEGER NULL CHECK (power_zone_1_max > 0),
    power_zone_2_max INTEGER NULL CHECK (power_zone_2_max > 0),
    power_zone_3_max INTEGER NULL CHECK (power_zone_3_max > 0),
    power_zone_4_max INTEGER NULL CHECK (power_zone_4_max > 0),
    power_zone_5_max INTEGER NULL CHECK (power_zone_5_max > 0),

    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. garmin_sourced_fields — the per-field source policy on the config row.
--    Default empty = all values manual (today's semantics preserved: the
--    effective view equals the confirmed config until a source is flipped).
--    Whitelisted tokens: ftp_watts, lactate_threshold_hr, max_hr,
--    threshold_pace_sec_per_km, hr_zones, power_zones (zones flip as groups).
--    Owned by the human side and mutated ONLY by PUT /athlete-config/sources;
--    the config PUT's full-replace deliberately does not touch this column.
ALTER TABLE athlete_config
    ADD COLUMN garmin_sourced_fields TEXT[] NOT NULL DEFAULT '{}';
