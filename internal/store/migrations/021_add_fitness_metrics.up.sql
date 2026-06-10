-- Daily fitness snapshot from a fitness source (Garmin today). One row per
-- calendar date; date-keyed upsert like recovery_metrics. Race predictions are
-- stored as whole seconds — the agent formats h:mm:ss. ACWR is not stored; it is
-- acute_load / chronic_load, derived at read time. All metrics nullable.

CREATE TABLE fitness_metrics (
    date                        DATE PRIMARY KEY,
    vo2max_running              NUMERIC(4,1) CHECK (vo2max_running IS NULL OR vo2max_running > 0),
    vo2max_cycling              NUMERIC(4,1) CHECK (vo2max_cycling IS NULL OR vo2max_cycling > 0),
    race_predictor_5k_seconds   INTEGER CHECK (race_predictor_5k_seconds IS NULL OR race_predictor_5k_seconds > 0),
    race_predictor_10k_seconds  INTEGER CHECK (race_predictor_10k_seconds IS NULL OR race_predictor_10k_seconds > 0),
    race_predictor_half_seconds INTEGER CHECK (race_predictor_half_seconds IS NULL OR race_predictor_half_seconds > 0),
    race_predictor_full_seconds INTEGER CHECK (race_predictor_full_seconds IS NULL OR race_predictor_full_seconds > 0),
    acute_load                  NUMERIC(6,1) CHECK (acute_load IS NULL OR acute_load >= 0),
    chronic_load                NUMERIC(6,1) CHECK (chronic_load IS NULL OR chronic_load >= 0),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
