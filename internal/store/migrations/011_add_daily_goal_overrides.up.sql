-- Per-date override of the default nutrition goals. Column shape mirrors
-- nutrition_goals after migration 008_unify_goals_shape — every nutrient is
-- {min?, max?}. One row per calendar date; no singleton, no FKs.

CREATE TABLE daily_goal_overrides (
    date DATE PRIMARY KEY,

    kcal_min            NUMERIC(10, 3),
    kcal_max            NUMERIC(10, 3),
    protein_g_min       NUMERIC(10, 3),
    protein_g_max       NUMERIC(10, 3),
    carbs_g_min         NUMERIC(10, 3),
    carbs_g_max         NUMERIC(10, 3),
    fat_g_min           NUMERIC(10, 3),
    fat_g_max           NUMERIC(10, 3),
    fiber_g_min         NUMERIC(10, 3),
    fiber_g_max         NUMERIC(10, 3),
    sugar_g_min         NUMERIC(10, 3),
    sugar_g_max         NUMERIC(10, 3),
    salt_g_min          NUMERIC(10, 3),
    salt_g_max          NUMERIC(10, 3),

    iron_mg_min         NUMERIC(10, 3),
    iron_mg_max         NUMERIC(10, 3),
    calcium_mg_min      NUMERIC(10, 3),
    calcium_mg_max      NUMERIC(10, 3),
    vitamin_d_mcg_min   NUMERIC(10, 3),
    vitamin_d_mcg_max   NUMERIC(10, 3),
    vitamin_b12_mcg_min NUMERIC(10, 3),
    vitamin_b12_mcg_max NUMERIC(10, 3),
    vitamin_c_mg_min    NUMERIC(10, 3),
    vitamin_c_mg_max    NUMERIC(10, 3),
    magnesium_mg_min    NUMERIC(10, 3),
    magnesium_mg_max    NUMERIC(10, 3),
    potassium_mg_min    NUMERIC(10, 3),
    potassium_mg_max    NUMERIC(10, 3),
    zinc_mg_min         NUMERIC(10, 3),
    zinc_mg_max         NUMERIC(10, 3),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
