-- In-session fueling layer (gels, electrolyte drinks, caffeine, salt tabs).
-- Sibling to hydration_entries — kept separate to avoid mixing ml with mg/g
-- in one Totals struct (the unit-isolation rationale from add-hydration-tracking).
-- At least one of {quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}
-- must be set; the service enforces it (a CHECK would be possible but harder
-- to give a friendly 400 from).

CREATE TABLE workout_fuel_entries (
    id            UUID PRIMARY KEY,
    logged_at     TIMESTAMPTZ NOT NULL,
    name          TEXT NOT NULL CHECK (length(name) > 0),
    quantity_ml   NUMERIC(10, 1) NULL CHECK (quantity_ml IS NULL OR quantity_ml > 0),
    carbs_g       NUMERIC(10, 1) NULL CHECK (carbs_g      IS NULL OR carbs_g      >= 0),
    sodium_mg     NUMERIC(10, 1) NULL CHECK (sodium_mg    IS NULL OR sodium_mg    >= 0),
    potassium_mg  NUMERIC(10, 1) NULL CHECK (potassium_mg IS NULL OR potassium_mg >= 0),
    caffeine_mg   NUMERIC(10, 1) NULL CHECK (caffeine_mg  IS NULL OR caffeine_mg  >= 0),
    note          TEXT NULL,
    workout_id    UUID NULL REFERENCES workouts(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workout_fuel_entries_logged_at_idx
    ON workout_fuel_entries (logged_at);

CREATE INDEX workout_fuel_entries_workout_id_idx
    ON workout_fuel_entries (workout_id)
    WHERE workout_id IS NOT NULL;
