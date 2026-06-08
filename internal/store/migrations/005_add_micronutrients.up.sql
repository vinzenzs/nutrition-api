ALTER TABLE products
    ADD COLUMN iron_mg_per_100g         NUMERIC(10, 3),
    ADD COLUMN calcium_mg_per_100g      NUMERIC(10, 3),
    ADD COLUMN vitamin_d_mcg_per_100g   NUMERIC(10, 3),
    ADD COLUMN vitamin_b12_mcg_per_100g NUMERIC(10, 3),
    ADD COLUMN vitamin_c_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN magnesium_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN potassium_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN zinc_mg_per_100g         NUMERIC(10, 3);

ALTER TABLE meal_entries
    ADD COLUMN snapshot_iron_mg_per_100g         NUMERIC(10, 3),
    ADD COLUMN snapshot_calcium_mg_per_100g      NUMERIC(10, 3),
    ADD COLUMN snapshot_vitamin_d_mcg_per_100g   NUMERIC(10, 3),
    ADD COLUMN snapshot_vitamin_b12_mcg_per_100g NUMERIC(10, 3),
    ADD COLUMN snapshot_vitamin_c_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN snapshot_magnesium_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN snapshot_potassium_mg_per_100g    NUMERIC(10, 3),
    ADD COLUMN snapshot_zinc_mg_per_100g         NUMERIC(10, 3);
