-- Reverse the widen. LOSSY: rows that can't be represented in the old
-- recommendation-only shape (non-recommendation kinds, or recommendations that
-- were left dateless/scopeless under the relaxed schema) are dropped before the
-- NOT NULL constraints are re-tightened.
DELETE FROM coach_memory
    WHERE kind <> 'recommendation' OR date IS NULL OR scope IS NULL;

DROP INDEX IF EXISTS coach_memory_status_idx;
ALTER TABLE coach_memory DROP CONSTRAINT IF EXISTS coach_memory_recommendation_date_check;
ALTER TABLE coach_memory DROP CONSTRAINT IF EXISTS coach_memory_kind_check;

ALTER TABLE coach_memory DROP COLUMN kind;
ALTER TABLE coach_memory DROP COLUMN expires_at;
ALTER TABLE coach_memory DROP COLUMN review_at;
ALTER TABLE coach_memory DROP COLUMN status;

ALTER TABLE coach_memory ALTER COLUMN date  SET NOT NULL;
ALTER TABLE coach_memory ALTER COLUMN scope SET NOT NULL;

ALTER INDEX coach_memory_date_idx RENAME TO coach_recommendations_date_idx;
ALTER TABLE coach_memory RENAME COLUMN text TO recommendation;
ALTER TABLE coach_memory RENAME TO coach_recommendations;
