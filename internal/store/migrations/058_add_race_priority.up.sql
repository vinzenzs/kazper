-- A/B/C race priority (add-race-priority). TrainingPeaks-style triage the coach
-- agent reasons over. Nullable, no default, no back-fill: an absent priority
-- means "not yet triaged" — honest for existing rows (the training_focus
-- precedent). CHECK enforces the closed set at the DB boundary.
ALTER TABLE races
    ADD COLUMN priority TEXT NULL CHECK (priority IN ('A', 'B', 'C'));
