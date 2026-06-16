-- Multisport plan integration (per multisport-phase-2). A plan slot may now
-- reference EITHER a single-sport workout_templates row OR a multisport_templates
-- row (exactly one). And a materialized workout may be a multisport brick row
-- (sport='multisport', referencing the multisport template). transition is NOT a
-- workout sport — it only appears as a segment sport inside a multisport template.

-- plan_slots: template_id becomes nullable; add multisport_template_id; XOR check.
ALTER TABLE plan_slots
    ALTER COLUMN template_id DROP NOT NULL;

ALTER TABLE plan_slots
    ADD COLUMN multisport_template_id UUID NULL REFERENCES multisport_templates(id) ON DELETE RESTRICT;

ALTER TABLE plan_slots
    ADD CONSTRAINT plan_slots_template_xor_check
    CHECK ((template_id IS NULL) <> (multisport_template_id IS NULL));

-- workouts: add the multisport template link and widen the sport vocabulary.
ALTER TABLE workouts
    ADD COLUMN multisport_template_id UUID NULL REFERENCES multisport_templates(id) ON DELETE SET NULL;

ALTER TABLE workouts
    DROP CONSTRAINT workouts_sport_check;

ALTER TABLE workouts
    ADD CONSTRAINT workouts_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'multisport', 'other'));
