CREATE TABLE products (
    id              UUID PRIMARY KEY,
    barcode         TEXT UNIQUE,
    name            TEXT NOT NULL,
    brand           TEXT,
    source          TEXT NOT NULL CHECK (source IN ('off', 'manual')),

    kcal_per_100g       NUMERIC(10, 3),
    protein_g_per_100g  NUMERIC(10, 3),
    carbs_g_per_100g    NUMERIC(10, 3),
    fat_g_per_100g      NUMERIC(10, 3),
    fiber_g_per_100g    NUMERIC(10, 3),
    sugar_g_per_100g    NUMERIC(10, 3),
    salt_g_per_100g     NUMERIC(10, 3),

    serving_size_g  NUMERIC(10, 3),
    off_payload     JSONB,

    fetched_at      TIMESTAMPTZ,
    last_logged_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
