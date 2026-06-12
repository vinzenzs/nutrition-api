-- add-garmin-workout-detail (change B): bring per-activity detail Garmin exposes
-- into the workouts capability — the fueling math needs duration-at-intensity,
-- not a single average HR. Three additions, all nullable / additive:
--
--   1. Scalar performance + weather columns on `workouts` (ride along the
--      activity summary the bridge already fetches; weather is one extra call).
--   2. Fixed HR-zone time columns `secs_in_zone_1..5` (always 5 buckets →
--      columns, not a child table — the most-queried fueling signal, join-free).
--   3. Child tables `workout_splits` (per-lap) and `workout_sets` (strength),
--      both 1:N with ON DELETE CASCADE.
--
-- All scalar/zone/weather columns are NULL with no back-fill — "not measured"
-- stays a meaningful state, mirroring the rpe/gi and ingestion-metric precedent.
-- CHECKs give defence-in-depth alongside handler-level validation.

ALTER TABLE workouts
    ADD COLUMN elevation_gain_m   NUMERIC(8, 1) CHECK (elevation_gain_m IS NULL OR elevation_gain_m >= 0),
    ADD COLUMN elevation_loss_m   NUMERIC(8, 1) CHECK (elevation_loss_m IS NULL OR elevation_loss_m >= 0),
    ADD COLUMN normalized_power_w INTEGER       CHECK (normalized_power_w IS NULL OR normalized_power_w > 0),
    ADD COLUMN intensity_factor   NUMERIC(4, 2) CHECK (intensity_factor IS NULL OR intensity_factor >= 0),
    ADD COLUMN avg_cadence        INTEGER       CHECK (avg_cadence IS NULL OR avg_cadence > 0),
    ADD COLUMN avg_stride_m       NUMERIC(5, 2) CHECK (avg_stride_m IS NULL OR avg_stride_m > 0),
    ADD COLUMN max_hr             INTEGER       CHECK (max_hr IS NULL OR max_hr > 0),
    ADD COLUMN aerobic_te         NUMERIC(3, 1) CHECK (aerobic_te IS NULL OR aerobic_te >= 0),
    ADD COLUMN anaerobic_te       NUMERIC(3, 1) CHECK (anaerobic_te IS NULL OR anaerobic_te >= 0),
    ADD COLUMN secs_in_zone_1     INTEGER       CHECK (secs_in_zone_1 IS NULL OR secs_in_zone_1 >= 0),
    ADD COLUMN secs_in_zone_2     INTEGER       CHECK (secs_in_zone_2 IS NULL OR secs_in_zone_2 >= 0),
    ADD COLUMN secs_in_zone_3     INTEGER       CHECK (secs_in_zone_3 IS NULL OR secs_in_zone_3 >= 0),
    ADD COLUMN secs_in_zone_4     INTEGER       CHECK (secs_in_zone_4 IS NULL OR secs_in_zone_4 >= 0),
    ADD COLUMN secs_in_zone_5     INTEGER       CHECK (secs_in_zone_5 IS NULL OR secs_in_zone_5 >= 0),
    ADD COLUMN humidity_pct       NUMERIC(5, 1) CHECK (humidity_pct IS NULL OR (humidity_pct BETWEEN 0 AND 100)),
    ADD COLUMN wind_speed_mps     NUMERIC(5, 1) CHECK (wind_speed_mps IS NULL OR wind_speed_mps >= 0);

-- Per-lap splits (1:N). Endurance activities populate these; counts vary, so a
-- child table rather than columns. CASCADE so deleting a workout reaps its laps.
CREATE TABLE workout_splits (
    id               UUID PRIMARY KEY,
    workout_id       UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    split_index      INTEGER NOT NULL CHECK (split_index >= 0),
    distance_m       NUMERIC(10, 1),
    duration_s       NUMERIC(10, 1),
    avg_hr           INTEGER,
    avg_power_w      INTEGER,
    avg_speed_mps    NUMERIC(8, 3),
    elevation_gain_m NUMERIC(8, 1)
);

CREATE INDEX workout_splits_workout_id_idx ON workout_splits (workout_id);
CREATE UNIQUE INDEX workout_splits_workout_id_split_index_uidx
    ON workout_splits (workout_id, split_index);

-- Per-set strength data (1:N), filling the previously-blank strength sessions.
CREATE TABLE workout_sets (
    id                UUID PRIMARY KEY,
    workout_id        UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    set_index         INTEGER NOT NULL CHECK (set_index >= 0),
    exercise_name     TEXT,
    exercise_category TEXT,
    reps              INTEGER       CHECK (reps IS NULL OR reps >= 0),
    weight_kg         NUMERIC(6, 2) CHECK (weight_kg IS NULL OR weight_kg >= 0),
    duration_s        NUMERIC(10, 1)
);

CREATE INDEX workout_sets_workout_id_idx ON workout_sets (workout_id);
CREATE UNIQUE INDEX workout_sets_workout_id_set_index_uidx
    ON workout_sets (workout_id, set_index);
