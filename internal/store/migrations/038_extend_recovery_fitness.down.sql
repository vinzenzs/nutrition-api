ALTER TABLE fitness_metrics
    DROP COLUMN IF EXISTS endurance_score,
    DROP COLUMN IF EXISTS hill_score,
    DROP COLUMN IF EXISTS fitness_age,
    DROP COLUMN IF EXISTS training_status;

ALTER TABLE recovery_metrics
    DROP COLUMN IF EXISTS spo2_avg,
    DROP COLUMN IF EXISTS spo2_lowest,
    DROP COLUMN IF EXISTS respiration_avg,
    DROP COLUMN IF EXISTS respiration_lowest,
    DROP COLUMN IF EXISTS deep_sleep_seconds,
    DROP COLUMN IF EXISTS light_sleep_seconds,
    DROP COLUMN IF EXISTS rem_sleep_seconds,
    DROP COLUMN IF EXISTS awake_seconds;
