-- Training phases (T1 #1A) and goal templates (T1 #5) shipped together.
-- Templates are reusable goal-sets (same 30-column nutrient projection as
-- daily_goal_overrides). Phases are named date ranges tagged with a training
-- type; an optional default_template_id links to a template whose bounds
-- drive adherence on dates within the phase (resolver-time, not materialized).
--
-- The effective-goals chain after this migration: per-date override >
-- phase's default template > singleton default. See `nutrition-goals` spec.

CREATE TABLE goal_templates (
    id   UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
         CHECK (length(trim(name)) > 0 AND length(name) <= 128),
    notes TEXT,

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

CREATE TABLE training_phases (
    id   UUID PRIMARY KEY,
    name TEXT NOT NULL
         CHECK (length(trim(name)) > 0 AND length(name) <= 128),
    -- TEXT + CHECK over Postgres ENUM so adding values later is a single
    -- migration without ALTER TYPE's non-transactional issues. Matches the
    -- pattern from workouts.source / workouts.sport.
    type TEXT NOT NULL
         CHECK (type IN ('base', 'build', 'peak', 'recovery', 'race_week', 'off_season', 'other')),
    start_date DATE NOT NULL,
    end_date   DATE NOT NULL CHECK (end_date >= start_date),

    -- ON DELETE RESTRICT: deleting a template referenced by a phase is
    -- refused with a 409 at the handler boundary. SET NULL was considered
    -- and rejected (silent data loss; the phase would suddenly resolve to
    -- the singleton without the user knowing).
    default_template_id UUID REFERENCES goal_templates(id) ON DELETE RESTRICT,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Range intersection is the hot query (the resolver issues
-- `WHERE start_date <= $to AND end_date >= $from`); a composite B-tree on
-- both columns covers it.
CREATE INDEX training_phases_date_range_idx
    ON training_phases (start_date, end_date);
