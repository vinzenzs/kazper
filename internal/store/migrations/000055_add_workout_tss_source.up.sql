-- Provenance for workouts.tss: how the value was obtained. Server-managed;
-- nullable. Paired with tss (a TSS without provenance, or vice versa, is a bug).
ALTER TABLE workouts
    ADD COLUMN tss_source TEXT NULL
    CHECK (tss_source IN ('garmin', 'manual', 'power', 'pace', 'hr'));

-- Back-fill existing rows that already carry a tss: Garmin-sourced load is a
-- measured/watch value ('garmin'), anything else is a caller-supplied ('manual').
-- source is NOT NULL, so this CASE is total over the tss-NOT-NULL rows and makes
-- the pairing CHECK below satisfiable on existing data.
UPDATE workouts
SET tss_source = CASE WHEN source = 'garmin' THEN 'garmin' ELSE 'manual' END
WHERE tss IS NOT NULL;

-- Pairing invariant: tss and tss_source are both NULL or both set.
ALTER TABLE workouts
    ADD CONSTRAINT workouts_tss_source_pairing
    CHECK ((tss IS NULL) = (tss_source IS NULL));
