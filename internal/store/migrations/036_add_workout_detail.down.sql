-- Reverse 036: drop child tables first (FKs), then the added columns. Additive
-- rollback — no data transform; existing rows read back unchanged.
DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workout_splits;

ALTER TABLE workouts
    DROP COLUMN IF EXISTS elevation_gain_m,
    DROP COLUMN IF EXISTS elevation_loss_m,
    DROP COLUMN IF EXISTS normalized_power_w,
    DROP COLUMN IF EXISTS intensity_factor,
    DROP COLUMN IF EXISTS avg_cadence,
    DROP COLUMN IF EXISTS avg_stride_m,
    DROP COLUMN IF EXISTS max_hr,
    DROP COLUMN IF EXISTS aerobic_te,
    DROP COLUMN IF EXISTS anaerobic_te,
    DROP COLUMN IF EXISTS secs_in_zone_1,
    DROP COLUMN IF EXISTS secs_in_zone_2,
    DROP COLUMN IF EXISTS secs_in_zone_3,
    DROP COLUMN IF EXISTS secs_in_zone_4,
    DROP COLUMN IF EXISTS secs_in_zone_5,
    DROP COLUMN IF EXISTS humidity_pct,
    DROP COLUMN IF EXISTS wind_speed_mps;
