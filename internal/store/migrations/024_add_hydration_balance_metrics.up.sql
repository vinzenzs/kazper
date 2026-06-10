-- Daily water-balance snapshot from a hydration source (Garmin's daily hydration
-- response carries sweatLossInML + activityIntakeInML + goalInML). One row per
-- calendar date; date-keyed upsert like recovery_metrics / fitness_metrics. All
-- metrics nullable — NULL means "not reported." DISTINCT from the `hydration`
-- capability (per-entry logged intake): this is the device's daily estimate.
--
-- No total_intake_ml column on purpose — Garmin's daily valueInML is already
-- pushed to /hydration as a per-day entry; the daily balance reads total-in from
-- the hydration summary and sweat-out from here.

CREATE TABLE hydration_balance_metrics (
    date               DATE PRIMARY KEY,
    sweat_loss_ml      NUMERIC(10,1) CHECK (sweat_loss_ml IS NULL OR sweat_loss_ml > 0),
    activity_intake_ml NUMERIC(10,1) CHECK (activity_intake_ml IS NULL OR activity_intake_ml >= 0),
    goal_ml            NUMERIC(10,1) CHECK (goal_ml IS NULL OR goal_ml > 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
