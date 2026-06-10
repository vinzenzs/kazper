-- Planned/completed lifecycle on workouts. Existing rows are all completed
-- activities, so the column DEFAULT back-fills them to 'completed'. A planned
-- workout (from the Garmin training calendar) may be future-dated — the
-- future-date guard is relaxed for status='planned' at the service layer.

ALTER TABLE workouts
    ADD COLUMN status TEXT NOT NULL DEFAULT 'completed'
        CHECK (status IN ('planned', 'completed'));

CREATE INDEX workouts_status_idx ON workouts (status);
