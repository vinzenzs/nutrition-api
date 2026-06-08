CREATE TABLE nutrition_goals (
    id UUID PRIMARY KEY DEFAULT '00000000-0000-0000-0000-000000000001'::uuid,

    kcal_target          NUMERIC(10, 3),

    protein_g_min        NUMERIC(10, 3),
    protein_g_max        NUMERIC(10, 3),
    carbs_g_min          NUMERIC(10, 3),
    carbs_g_max          NUMERIC(10, 3),
    fat_g_min            NUMERIC(10, 3),
    fat_g_max            NUMERIC(10, 3),
    fiber_g_min          NUMERIC(10, 3),
    sugar_g_max          NUMERIC(10, 3),
    salt_g_max           NUMERIC(10, 3),

    iron_mg_min          NUMERIC(10, 3),
    calcium_mg_min       NUMERIC(10, 3),
    vitamin_d_mcg_min    NUMERIC(10, 3),
    vitamin_b12_mcg_min  NUMERIC(10, 3),
    vitamin_c_mg_min     NUMERIC(10, 3),
    magnesium_mg_min     NUMERIC(10, 3),
    potassium_mg_min     NUMERIC(10, 3),
    zinc_mg_min          NUMERIC(10, 3),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT nutrition_goals_singleton
        CHECK (id = '00000000-0000-0000-0000-000000000001'::uuid)
);
