-- Widen the shared sport vocabulary to admit 'yoga' and 'mobility' so standalone
-- yoga/mobility sessions keep their real sport end-to-end (table CHECK on both
-- workouts and workout_templates). Inline column CHECKs from 012/030 carry the
-- auto-generated *_sport_check names.
ALTER TABLE workouts
    DROP CONSTRAINT workouts_sport_check;

ALTER TABLE workouts
    ADD CONSTRAINT workouts_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other'));

ALTER TABLE workout_templates
    DROP CONSTRAINT workout_templates_sport_check;

ALTER TABLE workout_templates
    ADD CONSTRAINT workout_templates_sport_check
    CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other'));
