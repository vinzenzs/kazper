-- Macrocycle planning (per add-macrocycle-planning): the season-level
-- periodization container that orders existing training-phases into a yearly
-- progression toward a goal race. A macrocycle is planning/visualization +
-- coach-context only — it never enters the goals resolver or plan
-- materialization. Phases remain the single source of truth for the per-period
-- date ranges; a macrocycle stores only the season envelope.

CREATE TABLE macrocycles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    -- The A-race the season builds toward. SET NULL (not CASCADE) so deleting a
    -- race only drops the anchor, never the season — mirrors training_plans.race_id.
    race_id     UUID NULL REFERENCES races(id) ON DELETE SET NULL,
    -- Curated, cited Markdown "why this whole arc" prose, stored verbatim;
    -- distinct from the operational notes (mirrors training_phases.methodology).
    methodology TEXT NULL,
    notes       TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (start_date <= end_date)
);

-- Phase membership + per-period progression targets (the "group existing
-- phases" model). All four are nullable with no DEFAULT and no back-fill: an
-- unlinked phase with no targets is a valid state and existing adherence
-- behavior is unchanged. macrocycle_id is SET NULL on season delete so the
-- phases survive (orphaned, still driving adherence).
ALTER TABLE training_phases
    ADD COLUMN macrocycle_id        UUID NULL REFERENCES macrocycles(id) ON DELETE SET NULL,
    ADD COLUMN macrocycle_ordinal   INTEGER NULL,
    ADD COLUMN target_weekly_tss    NUMERIC NULL CHECK (target_weekly_tss IS NULL OR target_weekly_tss >= 0),
    ADD COLUMN target_weekly_hours  NUMERIC NULL CHECK (target_weekly_hours IS NULL OR target_weekly_hours >= 0);

-- Indexes for the covering-date query and the member-phase aggregate read.
CREATE INDEX macrocycles_date_range_idx ON macrocycles (start_date, end_date);
CREATE INDEX training_phases_macrocycle_id_idx ON training_phases (macrocycle_id);
