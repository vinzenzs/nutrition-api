DROP INDEX IF EXISTS hydration_entries_workout_id_idx;
DROP INDEX IF EXISTS meal_entries_workout_id_idx;
ALTER TABLE hydration_entries DROP COLUMN IF EXISTS workout_id;
ALTER TABLE meal_entries DROP COLUMN IF EXISTS workout_id;
