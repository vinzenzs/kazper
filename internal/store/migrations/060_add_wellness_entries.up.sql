-- Subjective daily wellness log (add-wellness-diary). One row per calendar date,
-- five OPTIONAL self-reported 1-5 scores plus an optional free-text note. The
-- objective recovery picture (HRV/sleep/RHR/readiness) is already Garmin-fed and
-- lives elsewhere; this holds only the self-reported half the coach collects in
-- conversation. Per-date singleton (PK on entry_date), PUT full-replace upsert.
-- Symptom-like fields read 1 = none -> 5 = severe; state-like fields read
-- 1 = low -> 5 = high (each keeps its natural conversational direction). At
-- least one field must be present -- the empty-entry rule is enforced in the
-- service, not the schema (a row with every score null but a note is valid).
CREATE TABLE wellness_entries (
    entry_date DATE PRIMARY KEY,

    fatigue    SMALLINT NULL CHECK (fatigue    BETWEEN 1 AND 5),
    soreness   SMALLINT NULL CHECK (soreness   BETWEEN 1 AND 5),
    stress     SMALLINT NULL CHECK (stress     BETWEEN 1 AND 5),
    mood       SMALLINT NULL CHECK (mood       BETWEEN 1 AND 5),
    motivation SMALLINT NULL CHECK (motivation BETWEEN 1 AND 5),
    note       TEXT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
