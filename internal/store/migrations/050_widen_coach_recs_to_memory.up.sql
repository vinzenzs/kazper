-- Widen the coach-recommendation log into a general coach-memory store
-- (widen-coach-recs-to-memory). A recommendation becomes one `kind` of memory
-- alongside dateless standing items (fact / preference / constraint /
-- observation), with a review/expire lifecycle. The store stays a primitive:
-- it records authored text verbatim, never synthesizes, and never mutates an
-- enforced target. Existing rows back-fill to kind='recommendation'.

ALTER TABLE coach_recommendations RENAME TO coach_memory;
ALTER TABLE coach_memory RENAME COLUMN recommendation TO text;

-- kind discriminator; back-fill existing rows as recommendations, then enforce.
ALTER TABLE coach_memory ADD COLUMN kind TEXT;
UPDATE coach_memory SET kind = 'recommendation';
ALTER TABLE coach_memory ALTER COLUMN kind SET NOT NULL;
ALTER TABLE coach_memory ADD CONSTRAINT coach_memory_kind_check
    CHECK (kind IN ('fact', 'preference', 'constraint', 'observation', 'recommendation'));

-- Lifecycle fields. status defaults active (back-fills existing rows).
ALTER TABLE coach_memory ADD COLUMN expires_at DATE;
ALTER TABLE coach_memory ADD COLUMN review_at  DATE;
ALTER TABLE coach_memory ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'archived'));

-- Demote date + scope from required to optional metadata. A NULL scope passes
-- the existing scope IN (...) CHECK (NULL IN (...) is unknown, which a CHECK
-- treats as satisfied).
ALTER TABLE coach_memory ALTER COLUMN date  DROP NOT NULL;
ALTER TABLE coach_memory ALTER COLUMN scope DROP NOT NULL;

-- A recommendation still requires a date (enforced in the app too); standing
-- kinds may be dateless.
ALTER TABLE coach_memory ADD CONSTRAINT coach_memory_recommendation_date_check
    CHECK (kind <> 'recommendation' OR date IS NOT NULL);

-- Grounding reads filter on status + expiry and order newest-first.
ALTER INDEX coach_recommendations_date_idx RENAME TO coach_memory_date_idx;
CREATE INDEX coach_memory_status_idx ON coach_memory (status, kind, date DESC, created_at DESC);
