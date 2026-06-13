-- add-garmin-misc-mirror (change G): the arc's catch-all tail — device inventory,
-- daily health vitals (blood pressure + all-day HR/stress), and earned
-- badges/challenges. Completeness-driven, LOW PRIORITY: none of these feed the
-- fueling/EA/hydration math; they are coaching/reference context, unit-isolated
-- (never merged into nutrition Totals). devices + achievements are upsert-by-
-- external-id inventory; health_vitals is a date-keyed daily snapshot.

CREATE TABLE devices (
    id               UUID PRIMARY KEY,
    external_id      TEXT NOT NULL,
    display_name     TEXT NOT NULL,
    model            TEXT,
    last_sync_at     TIMESTAMPTZ,
    battery_pct      NUMERIC(5, 1) CHECK (battery_pct IS NULL OR (battery_pct >= 0 AND battery_pct <= 100)),
    firmware_version TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX devices_external_id_uidx ON devices (external_id);

CREATE TABLE health_vitals (
    date         DATE PRIMARY KEY,
    bp_systolic  INTEGER CHECK (bp_systolic IS NULL OR bp_systolic > 0),
    bp_diastolic INTEGER CHECK (bp_diastolic IS NULL OR bp_diastolic > 0),
    bp_pulse     INTEGER CHECK (bp_pulse IS NULL OR bp_pulse > 0),
    resting_hr   INTEGER CHECK (resting_hr IS NULL OR resting_hr > 0),
    min_hr       INTEGER CHECK (min_hr IS NULL OR min_hr > 0),
    max_hr       INTEGER CHECK (max_hr IS NULL OR max_hr > 0),
    stress_avg   INTEGER CHECK (stress_avg IS NULL OR (stress_avg BETWEEN 0 AND 100)),
    stress_max   INTEGER CHECK (stress_max IS NULL OR (stress_max BETWEEN 0 AND 100)),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE achievements (
    id           UUID PRIMARY KEY,
    external_id  TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('badge', 'challenge')),
    name         TEXT NOT NULL,
    earned_at    TIMESTAMPTZ,
    progress_pct NUMERIC(5, 1) CHECK (progress_pct IS NULL OR (progress_pct >= 0 AND progress_pct <= 100)),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX achievements_external_id_uidx ON achievements (external_id);
