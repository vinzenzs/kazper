-- Workout training-focus classification (per add-workout-training-focus).
-- Classifies a session's intensity band against the 7-zone German
-- Trainingsbereiche model (REKOM/GA1/GA2/EB/WSA/SB/KA). Nullable: an
-- unclassified session is a valid state, not a data-quality defect — so the
-- column is added with no DEFAULT and no back-fill, exactly like rpe/
-- gi_distress_score and the ingestion metrics. NULL passes the CHECK; the
-- closed value set is kept in sync with the service-layer ValidTrainingFocus
-- switch. The field is declared intent (set by the user/coach), independent of
-- the measured secs_in_zone_* actuals.
ALTER TABLE workouts ADD COLUMN training_focus TEXT
    CHECK (training_focus IN (
        'recovery',
        'basic_endurance_1',
        'basic_endurance_2',
        'development',
        'competition_specific',
        'peak',
        'strength_endurance'
    ));
