-- Per-leg manual pacing overrides (add-race-pacing-plan). The pacing plan is
-- otherwise compute-on-read; only negotiated overrides persist. Keyed by
-- (race_id, ordinal) — NOT race_legs.id — so an override survives the wholesale
-- leg replace PATCH /races/{id} performs, as long as the ordinal persists.
-- Cascades on race delete.
CREATE TABLE race_leg_pacing_overrides (
    race_id                       UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE,
    ordinal                       INTEGER NOT NULL,
    -- Exactly one unit family is populated (both low AND high), matching the
    -- leg's discipline: power (bike), sec_per_km (run), sec_per_100m (swim).
    target_power_low_w            INTEGER NULL,
    target_power_high_w           INTEGER NULL,
    target_pace_low_sec_per_km    NUMERIC NULL,
    target_pace_high_sec_per_km   NUMERIC NULL,
    target_pace_low_sec_per_100m  NUMERIC NULL,
    target_pace_high_sec_per_100m NUMERIC NULL,
    note                          TEXT NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (race_id, ordinal),

    -- Each family is all-or-nothing (low and high set together, or both null).
    CONSTRAINT pacing_override_power_pair CHECK (
        (target_power_low_w IS NULL) = (target_power_high_w IS NULL)
    ),
    CONSTRAINT pacing_override_km_pair CHECK (
        (target_pace_low_sec_per_km IS NULL) = (target_pace_high_sec_per_km IS NULL)
    ),
    CONSTRAINT pacing_override_100m_pair CHECK (
        (target_pace_low_sec_per_100m IS NULL) = (target_pace_high_sec_per_100m IS NULL)
    ),

    -- Exactly one family populated.
    CONSTRAINT pacing_override_one_family CHECK (
        (CASE WHEN target_power_low_w IS NOT NULL THEN 1 ELSE 0 END)
      + (CASE WHEN target_pace_low_sec_per_km IS NOT NULL THEN 1 ELSE 0 END)
      + (CASE WHEN target_pace_low_sec_per_100m IS NOT NULL THEN 1 ELSE 0 END)
      = 1
    ),

    -- Positive values, low <= high, per family.
    CONSTRAINT pacing_override_power_valid CHECK (
        target_power_low_w IS NULL
        OR (target_power_low_w > 0 AND target_power_high_w > 0 AND target_power_low_w <= target_power_high_w)
    ),
    CONSTRAINT pacing_override_km_valid CHECK (
        target_pace_low_sec_per_km IS NULL
        OR (target_pace_low_sec_per_km > 0 AND target_pace_high_sec_per_km > 0 AND target_pace_low_sec_per_km <= target_pace_high_sec_per_km)
    ),
    CONSTRAINT pacing_override_100m_valid CHECK (
        target_pace_low_sec_per_100m IS NULL
        OR (target_pace_low_sec_per_100m > 0 AND target_pace_high_sec_per_100m > 0 AND target_pace_low_sec_per_100m <= target_pace_high_sec_per_100m)
    )
);
