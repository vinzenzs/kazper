-- Training plan as a system of record (per add-training-plan): a plan owns
-- ordered weeks, each week owns ordered day-slots, each slot points at a
-- workout_template for a weekday. A materialize step (in the service layer)
-- expands this into dated, planned `workouts` rows keyed by plan_slot_id.

CREATE TABLE training_plans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    race_id    UUID NULL REFERENCES races(id) ON DELETE SET NULL,
    start_date DATE NOT NULL,                 -- the Monday of week 1
    notes      TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plan_weeks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id    UUID NOT NULL REFERENCES training_plans(id) ON DELETE CASCADE,
    ordinal    INTEGER NOT NULL CHECK (ordinal >= 1),   -- week 1..N
    phase_id   UUID NULL REFERENCES training_phases(id) ON DELETE SET NULL,
    notes      TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id, ordinal)
);

CREATE TABLE plan_slots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_week_id UUID NOT NULL REFERENCES plan_weeks(id) ON DELETE CASCADE,
    weekday      SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),  -- 0=Mon … 6=Sun
    ordinal      SMALLINT NOT NULL DEFAULT 0,            -- order of sessions within a day
    template_id  UUID NOT NULL REFERENCES workout_templates(id) ON DELETE RESTRICT,
    time_of_day  TIME NULL,                              -- optional; default applied at materialize
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_plan_weeks_plan_id ON plan_weeks (plan_id);
CREATE INDEX idx_plan_slots_plan_week_id ON plan_slots (plan_week_id);

-- workouts gains the two plan links. A planned-from-plan workout carries both;
-- an imported Garmin activity carries external_id and neither — disjoint
-- populations, so the slot-keyed upsert and the external_id upsert never
-- collide. SET NULL on delete preserves the workout row (history) when a slot
-- or template is removed.
ALTER TABLE workouts
    ADD COLUMN template_id  UUID NULL REFERENCES workout_templates(id) ON DELETE SET NULL,
    ADD COLUMN plan_slot_id UUID NULL REFERENCES plan_slots(id)        ON DELETE SET NULL;

-- The materialize upsert is keyed on plan_slot_id; the partial-unique index is
-- what makes "one planned workout per slot" enforceable and ON CONFLICT work.
CREATE UNIQUE INDEX workouts_plan_slot_id_key ON workouts (plan_slot_id) WHERE plan_slot_id IS NOT NULL;
