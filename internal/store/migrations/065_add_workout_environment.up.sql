-- Does ambient weather apply to this session? The enabler for the heat arc:
-- acclimatization only accrues outdoors, planned indoor sessions must not be
-- heat-adjusted, and EF-vs-temperature analytics must not be polluted by
-- trainer rides.
--
-- Nullable with NO back-fill (the training_focus precedent): null means "not
-- stated", and downstream heat logic treats it as assumed-outdoor and says so.
-- Inferring environment for existing rows from the incidental absence of
-- weather fields would invite wrong labels — null is the honest answer, and
-- the bridge's rolling re-sync window fills recent history for free.
ALTER TABLE workouts
    ADD COLUMN environment TEXT NULL
    CHECK (environment IN ('indoor', 'outdoor'));
