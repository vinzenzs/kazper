-- Dropping the column drops its inline enum CHECK with it. No data movement:
-- the column was additive and never back-filled.
ALTER TABLE workouts DROP COLUMN IF EXISTS environment;
