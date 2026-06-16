-- Revert multisport plan integration. Safe only if no multisport slots/workouts
-- exist (no rows with multisport_template_id set or sport='multisport'); remove
-- or remap such rows before rolling back.

-- workouts: narrow the sport vocabulary back and drop the template link.
ALTER TABLE workouts
    DROP CONSTRAINT workouts_sport_check;

ALTER TABLE workouts
    ADD CONSTRAINT workouts_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other'));

ALTER TABLE workouts
    DROP COLUMN multisport_template_id;

-- plan_slots: drop the XOR check and the multisport link, restore NOT NULL.
ALTER TABLE plan_slots
    DROP CONSTRAINT plan_slots_template_xor_check;

ALTER TABLE plan_slots
    DROP COLUMN multisport_template_id;

ALTER TABLE plan_slots
    ALTER COLUMN template_id SET NOT NULL;
