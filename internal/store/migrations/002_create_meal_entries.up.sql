CREATE TABLE meal_entries (
    id              UUID PRIMARY KEY,
    product_id      UUID REFERENCES products(id) ON DELETE SET NULL,
    logged_at       TIMESTAMPTZ NOT NULL,
    quantity_g      NUMERIC(10, 3) NOT NULL CHECK (quantity_g > 0),
    meal_type       TEXT CHECK (meal_type IS NULL OR meal_type IN ('breakfast', 'lunch', 'dinner', 'snack')),
    note            TEXT,

    snapshot_name              TEXT,
    snapshot_kcal_per_100g     NUMERIC(10, 3),
    snapshot_protein_g_per_100g NUMERIC(10, 3),
    snapshot_carbs_g_per_100g   NUMERIC(10, 3),
    snapshot_fat_g_per_100g     NUMERIC(10, 3),
    snapshot_fiber_g_per_100g   NUMERIC(10, 3),
    snapshot_sugar_g_per_100g   NUMERIC(10, 3),
    snapshot_salt_g_per_100g    NUMERIC(10, 3),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Either a product is linked or a snapshot name exists (freeform without product).
    CONSTRAINT meal_entries_requires_identity CHECK (product_id IS NOT NULL OR snapshot_name IS NOT NULL)
);
