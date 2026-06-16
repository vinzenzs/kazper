-- Coach recommendation log (per persist-coach-recommendations, priorities #6F).
-- A thin append-and-read primitive: the coach agent synthesizes a dated, scoped
-- recommendation and records it here; the API stores the authored text verbatim
-- and hands it back, exactly as `meals` records a user-logged intake. The server
-- never generates, ranks, or interprets a recommendation, and recording one does
-- NOT mutate any enforced target (goals/overrides remain the only path to that).
-- `date` is the day the advice applies to (agent-supplied). `scope` is a closed
-- set kept in sync with the service-layer validation. Corrections are delete +
-- re-log (no in-place edit), so there is no need for a richer history shape.
CREATE TABLE coach_recommendations (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date           DATE NOT NULL,
    scope          TEXT NOT NULL CHECK (scope IN ('fueling', 'training', 'recovery', 'race', 'general')),
    recommendation TEXT NOT NULL CHECK (length(recommendation) > 0),
    reason         TEXT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Windowed list ordered newest-first is the dominant read.
CREATE INDEX coach_recommendations_date_idx ON coach_recommendations (date DESC, created_at DESC);
