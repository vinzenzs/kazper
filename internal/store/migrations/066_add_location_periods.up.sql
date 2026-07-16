-- Where is the athlete on date X? (add-location-periods). The weather arc needs
-- (lat, lon, date) at city grade — not GPS tracks (a standing non-goal). The
-- athlete trains from home except while travelling, so home lives in config
-- (HOME_LAT/HOME_LON, quasi-static infrastructure like DEFAULT_USER_TZ) and
-- this table carries only the travel layer.
--
-- Inclusive date ranges. Overlaps are ACCEPTED on purpose: a weekend trip
-- nested inside a training camp is a real thing to log, and resolution picks
-- the covering period with the latest start_date (the macrocycle/public-feed
-- rule) rather than rejecting the write. No PATCH — corrections are delete +
-- re-log (the coach-memory precedent).
CREATE TABLE location_periods (
    id         UUID PRIMARY KEY,
    start_date DATE NOT NULL,
    end_date   DATE NOT NULL,
    name       TEXT NOT NULL,
    lat        DOUBLE PRECISION NOT NULL CHECK (lat >= -90 AND lat <= 90),
    lon        DOUBLE PRECISION NOT NULL CHECK (lon >= -180 AND lon <= 180),
    note       TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT location_periods_range_sane CHECK (end_date >= start_date)
);

-- Resolution walks periods covering a date (start_date <= d AND end_date >= d);
-- the window read filters on overlap. Both lead with start_date, which is also
-- the resolution tie-break.
CREATE INDEX location_periods_start_date_idx ON location_periods (start_date);
CREATE INDEX location_periods_end_date_idx ON location_periods (end_date);
