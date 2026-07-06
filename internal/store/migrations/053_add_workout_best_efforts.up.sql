-- strava-stats-frontend-phase-3 (effort-analytics): per-activity mean-maximal
-- best-effort records — the compact product computed from ingested per-second
-- power/speed streams. The raw streams are NOT persisted; only these best-mean
-- values per (workout, metric, duration) survive, which is exactly what a
-- power/pace curve is (a windowed MAX over these). Unit-isolated: power (W) and
-- speed (m/s) never feed any nutrition/hydration/energy total.
--
-- metric: 'power' (watts) for cycling, 'speed' (m/s) for run/swim — the frontend
-- renders speed as pace. value is the best rolling-window mean of that metric
-- over duration_s seconds anywhere in the activity ("best" = MAX for both).
-- achieved_at mirrors the workout's started_at so the curve can name the day.
-- The UNIQUE (workout_id, metric, duration_s) enables replace-on-repost.
CREATE TABLE workout_best_efforts (
    id          UUID PRIMARY KEY,
    workout_id  UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    metric      TEXT NOT NULL CHECK (metric IN ('power', 'speed')),
    duration_s  INTEGER NOT NULL CHECK (duration_s > 0),
    value       NUMERIC(12, 3) NOT NULL CHECK (value >= 0),
    achieved_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workout_best_efforts_uidx
    ON workout_best_efforts (workout_id, metric, duration_s);

-- The windowed curve query filters by metric + duration and picks the MAX value;
-- this index serves that GROUP-BY/DISTINCT-ON path.
CREATE INDEX workout_best_efforts_metric_dur_idx
    ON workout_best_efforts (metric, duration_s);
