-- Drop the pairing CHECK first, then the column (dropping the column also drops
-- its inline enum CHECK).
ALTER TABLE workouts DROP CONSTRAINT IF EXISTS workouts_tss_source_pairing;
ALTER TABLE workouts DROP COLUMN IF EXISTS tss_source;
