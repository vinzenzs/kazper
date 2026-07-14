-- Raw activity sample streams (persist-activity-streams). One row per
-- (workout, stream_type); the full contiguous 1 Hz series as a native REAL[]
-- (4 B/sample, TOAST-compressed). UNIQUE(workout_id, stream_type) makes a
-- re-post replace the stored series. Cascades on workout delete.
CREATE TABLE workout_streams (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_id     UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    stream_type    TEXT NOT NULL CHECK (stream_type IN ('power', 'speed', 'heart_rate')),
    samples        REAL[] NOT NULL,
    sample_rate_hz INTEGER NOT NULL DEFAULT 1,
    sample_count   INTEGER NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workout_id, stream_type)
);

CREATE INDEX workout_streams_workout_id_idx ON workout_streams (workout_id);

-- Stream-derived execution-quality metrics, written only by the stream
-- ingest/recompute path (never client-writable). Nullable, no back-fill —
-- historical rows have no stored streams until re-synced.
ALTER TABLE workouts
    ADD COLUMN variability_index NUMERIC(4, 2) NULL CHECK (variability_index IS NULL OR variability_index > 0),
    ADD COLUMN efficiency_factor NUMERIC(6, 3) NULL CHECK (efficiency_factor IS NULL OR efficiency_factor > 0),
    ADD COLUMN decoupling_pct    NUMERIC(5, 1) NULL CHECK (decoupling_pct IS NULL OR (decoupling_pct BETWEEN -100 AND 100));
