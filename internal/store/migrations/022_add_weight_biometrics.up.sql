-- Richer smart-scale biometrics on body-weight entries. All nullable, no
-- back-fill — existing rows read NULL (the scale didn't report these, a
-- meaningful state). Garmin's full weigh-in reports these alongside weight + BF%.

ALTER TABLE body_weight_entries
    ADD COLUMN muscle_mass_kg NUMERIC(5, 2)
        CHECK (muscle_mass_kg IS NULL OR muscle_mass_kg > 0);

ALTER TABLE body_weight_entries
    ADD COLUMN body_water_pct NUMERIC(4, 1)
        CHECK (body_water_pct IS NULL OR (body_water_pct >= 0 AND body_water_pct <= 100));

ALTER TABLE body_weight_entries
    ADD COLUMN bone_mass_kg NUMERIC(4, 2)
        CHECK (bone_mass_kg IS NULL OR bone_mass_kg > 0);

ALTER TABLE body_weight_entries
    ADD COLUMN bmi NUMERIC(4, 1)
        CHECK (bmi IS NULL OR bmi > 0);
