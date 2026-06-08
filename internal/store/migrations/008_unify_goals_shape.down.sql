-- Best-effort rollback: kcal_target is recovered as the midpoint of the
-- min/max range (lossy when the user set an asymmetric range), and the
-- added max columns for previously min-only nutrients (plus min columns
-- for previously max-only) are dropped.

ALTER TABLE nutrition_goals ADD COLUMN kcal_target NUMERIC(10, 3);

UPDATE nutrition_goals
SET kcal_target = ROUND((kcal_min + kcal_max) / 2, 3)
WHERE kcal_min IS NOT NULL AND kcal_max IS NOT NULL;

ALTER TABLE nutrition_goals
    DROP COLUMN kcal_min,
    DROP COLUMN kcal_max,
    DROP COLUMN fiber_g_max,
    DROP COLUMN sugar_g_min,
    DROP COLUMN salt_g_min,
    DROP COLUMN iron_mg_max,
    DROP COLUMN calcium_mg_max,
    DROP COLUMN vitamin_d_mcg_max,
    DROP COLUMN vitamin_b12_mcg_max,
    DROP COLUMN vitamin_c_mg_max,
    DROP COLUMN magnesium_mg_max,
    DROP COLUMN potassium_mg_max,
    DROP COLUMN zinc_mg_max;
