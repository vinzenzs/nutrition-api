CREATE TABLE planned_meals (
    id            UUID PRIMARY KEY,
    plan_date     DATE NOT NULL,
    slot          TEXT NOT NULL CHECK (slot IN ('breakfast', 'lunch', 'dinner', 'snack')),
    product_id    UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity_g    NUMERIC(10, 3) NULL CHECK (quantity_g IS NULL OR quantity_g > 0),
    status        TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'eaten', 'skipped')),
    -- Informational link to the meal_entries row created on the eaten
    -- transition. Deliberately NOT a FK: deleting a meal entry must not touch
    -- the plan row (see design D3).
    meal_entry_id UUID NULL,
    notes         TEXT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX planned_meals_plan_date_idx ON planned_meals (plan_date);
CREATE INDEX planned_meals_product_id_idx ON planned_meals (product_id);
