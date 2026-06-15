-- Multisport workout templates (per add-multisport-structured-workouts, Phase 1).
-- A multisport session — triathlon / brick — is an ordered list of per-sport
-- SEGMENTS (swim → T1 → bike → T2 → run), each carrying its own sport and its own
-- step program, plus transition (T1/T2) segments. Unlike workout_templates this
-- has no top-level sport: the sport lives per segment. Segments are a validated
-- JSONB array — always read whole, never queried individually — so no child
-- table; the service layer enforces ≥2 non-transition segments and validates each
-- segment's steps under that segment's sport. First-party authored: no
-- external_id/source. Single-sport workout_templates is untouched (the
-- single-sport invariant the resolver and the rest of the system rely on stays
-- intact); multisport lives in its own library + direct-schedule path.
CREATE TABLE multisport_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    segments    JSONB NOT NULL CHECK (jsonb_typeof(segments) = 'array' AND jsonb_array_length(segments) > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
