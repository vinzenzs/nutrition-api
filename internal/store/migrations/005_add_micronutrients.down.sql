ALTER TABLE meal_entries
    DROP COLUMN IF EXISTS snapshot_zinc_mg_per_100g,
    DROP COLUMN IF EXISTS snapshot_potassium_mg_per_100g,
    DROP COLUMN IF EXISTS snapshot_magnesium_mg_per_100g,
    DROP COLUMN IF EXISTS snapshot_vitamin_c_mg_per_100g,
    DROP COLUMN IF EXISTS snapshot_vitamin_b12_mcg_per_100g,
    DROP COLUMN IF EXISTS snapshot_vitamin_d_mcg_per_100g,
    DROP COLUMN IF EXISTS snapshot_calcium_mg_per_100g,
    DROP COLUMN IF EXISTS snapshot_iron_mg_per_100g;

ALTER TABLE products
    DROP COLUMN IF EXISTS zinc_mg_per_100g,
    DROP COLUMN IF EXISTS potassium_mg_per_100g,
    DROP COLUMN IF EXISTS magnesium_mg_per_100g,
    DROP COLUMN IF EXISTS vitamin_c_mg_per_100g,
    DROP COLUMN IF EXISTS vitamin_b12_mcg_per_100g,
    DROP COLUMN IF EXISTS vitamin_d_mcg_per_100g,
    DROP COLUMN IF EXISTS calcium_mg_per_100g,
    DROP COLUMN IF EXISTS iron_mg_per_100g;
