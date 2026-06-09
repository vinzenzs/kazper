-- Per-session rehearsal-outcome signals on workouts:
--   rpe                — Borg CR-10 perceived effort, 1..10
--   gi_distress_score  — sports-nutrition GI rehearsal severity, 1..5
--                        (1 = no distress, 5 = severe / had to stop)
--
-- Both nullable — not every workout is a fueling rehearsal (Z1 spins, manual
-- gym sessions, etc. don't need either). NULL means "not rehearsed/not
-- measured," a meaningful signal. CHECK constraints provide defence-in-depth
-- alongside the handler-level range validation.

ALTER TABLE workouts
    ADD COLUMN rpe INTEGER
        CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10));

ALTER TABLE workouts
    ADD COLUMN gi_distress_score INTEGER
        CHECK (gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5));
