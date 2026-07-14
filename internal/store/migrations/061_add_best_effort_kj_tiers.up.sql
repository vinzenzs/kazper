-- kJ-tiered best efforts (add-durability-analysis). The mean-maximal ladder was
-- fatigue-blind: a fresh 20-min best and one set 2000 kJ into a ride counted the
-- same. This widens workout_best_efforts with a kJ tier — tier 0 is exactly the
-- existing "fresh" ladder (after 0 kJ of work), and power series additionally
-- store 1m/5m/20m bests whose window starts after 500/1000/1500/2000 kJ of
-- accumulated work. The column defaults to 0 so every existing row and query
-- keeps its current meaning; the unique key widens to include the tier.
ALTER TABLE workout_best_efforts
    ADD COLUMN kj_tier SMALLINT NOT NULL DEFAULT 0 CHECK (kj_tier >= 0);

DROP INDEX workout_best_efforts_uidx;

CREATE UNIQUE INDEX workout_best_efforts_uidx
    ON workout_best_efforts (workout_id, metric, duration_s, kj_tier);
