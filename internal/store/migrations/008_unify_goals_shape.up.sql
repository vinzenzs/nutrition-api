-- Unify every nutrition_goals nutrient to the {min?, max?} shape.
-- kcal_target (scalar) is split into kcal_min and kcal_max with the prior
-- implicit ±5% rule frozen into literal values; min-only / max-only nutrients
-- gain their previously-missing bound column.

ALTER TABLE nutrition_goals
    ADD COLUMN kcal_min            NUMERIC(10, 3),
    ADD COLUMN kcal_max            NUMERIC(10, 3),
    ADD COLUMN fiber_g_max         NUMERIC(10, 3),
    ADD COLUMN sugar_g_min         NUMERIC(10, 3),
    ADD COLUMN salt_g_min          NUMERIC(10, 3),
    ADD COLUMN iron_mg_max         NUMERIC(10, 3),
    ADD COLUMN calcium_mg_max      NUMERIC(10, 3),
    ADD COLUMN vitamin_d_mcg_max   NUMERIC(10, 3),
    ADD COLUMN vitamin_b12_mcg_max NUMERIC(10, 3),
    ADD COLUMN vitamin_c_mg_max    NUMERIC(10, 3),
    ADD COLUMN magnesium_mg_max    NUMERIC(10, 3),
    ADD COLUMN potassium_mg_max    NUMERIC(10, 3),
    ADD COLUMN zinc_mg_max         NUMERIC(10, 3);

-- Backfill kcal: the prior ±5% tolerance becomes literal min/max so users keep
-- their effective targets after the change.
UPDATE nutrition_goals
SET kcal_min = ROUND(kcal_target * 0.95, 3),
    kcal_max = ROUND(kcal_target * 1.05, 3)
WHERE kcal_target IS NOT NULL;

ALTER TABLE nutrition_goals DROP COLUMN kcal_target;
