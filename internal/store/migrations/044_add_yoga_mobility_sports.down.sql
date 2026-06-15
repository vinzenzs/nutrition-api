-- Revert to the narrow sport vocabulary. Safe only if no rows use 'yoga' or
-- 'mobility'; delete or remap such rows before rolling back.
ALTER TABLE workout_templates
    DROP CONSTRAINT workout_templates_sport_check;

ALTER TABLE workout_templates
    ADD CONSTRAINT workout_templates_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'other'));

ALTER TABLE workouts
    DROP CONSTRAINT workouts_sport_check;

ALTER TABLE workouts
    ADD CONSTRAINT workouts_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'other'));
