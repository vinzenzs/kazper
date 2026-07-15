-- Timestamped supplement-intake log (add-supplement-log). Supplements (creatine,
-- iron, vitamin D, magnesium, out-of-session caffeine tablets) aren't meals (no
-- meaningful macros), aren't workout-fuel (not in-session), and freeform
-- coach-memory isn't queryable by date. A thin dated log answers "did the iron
-- protocol hold through the block?". Multiple entries per day are allowed. Dose
-- and its unit are paired (both or neither). Supplements feed no nutrition/
-- hydration/energy total (unit isolation).
CREATE TABLE supplement_entries (
    id         UUID PRIMARY KEY,
    logged_at  TIMESTAMPTZ NOT NULL,
    name       TEXT NOT NULL,
    dose       NUMERIC NULL CHECK (dose IS NULL OR dose > 0),
    dose_unit  TEXT NULL,
    note       TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- dose and dose_unit are present together or absent together.
    CONSTRAINT supplement_dose_paired CHECK ((dose IS NULL) = (dose_unit IS NULL))
);

CREATE INDEX supplement_entries_logged_at_idx ON supplement_entries (logged_at);
