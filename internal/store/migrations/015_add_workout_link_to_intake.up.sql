-- Link intake events (meals, hydration) to workouts. Nullable FK with
-- ON DELETE SET NULL: intake outlives workout deletion, like meal_entries
-- outlives product deletion. Partial indexes keep size minimal — most rows
-- have NULL here in v1.

ALTER TABLE meal_entries
    ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;

ALTER TABLE hydration_entries
    ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;

CREATE INDEX meal_entries_workout_id_idx
    ON meal_entries (workout_id)
    WHERE workout_id IS NOT NULL;

CREATE INDEX hydration_entries_workout_id_idx
    ON hydration_entries (workout_id)
    WHERE workout_id IS NOT NULL;
