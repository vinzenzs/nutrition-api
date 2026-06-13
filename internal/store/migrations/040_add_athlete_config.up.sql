-- add-garmin-athlete-config (change F): capture the athlete's slowly-changing
-- physiology configuration — FTP, threshold HR/paces, max HR, lactate-threshold
-- HR, and HR-zone (+ optional power-zone) boundaries — so change B's per-activity
-- normalized power and secs_in_zone_* become interpretable. Singleton row
-- (fixed sentinel PK, upsert-in-place), modeled on nutrition_goals — there is one
-- athlete with one current configuration, not a per-day snapshot. Every field is
-- nullable: NULL means "Garmin didn't provide it", distinct from a real zero.
-- CAPTURE ONLY — nothing consumes these values in this change.

CREATE TABLE athlete_config (
    id UUID PRIMARY KEY DEFAULT '00000000-0000-0000-0000-000000000001'::uuid,

    ftp_watts                        INTEGER        CHECK (ftp_watts IS NULL OR ftp_watts > 0),
    threshold_hr                     INTEGER        CHECK (threshold_hr IS NULL OR threshold_hr > 0),
    lactate_threshold_hr             INTEGER        CHECK (lactate_threshold_hr IS NULL OR lactate_threshold_hr > 0),
    max_hr                           INTEGER        CHECK (max_hr IS NULL OR max_hr > 0),
    threshold_pace_sec_per_km        NUMERIC(10, 3) CHECK (threshold_pace_sec_per_km IS NULL OR threshold_pace_sec_per_km > 0),
    threshold_swim_pace_sec_per_100m NUMERIC(10, 3) CHECK (threshold_swim_pace_sec_per_100m IS NULL OR threshold_swim_pace_sec_per_100m > 0),

    hr_zone_1_max INTEGER CHECK (hr_zone_1_max IS NULL OR hr_zone_1_max > 0),
    hr_zone_2_max INTEGER CHECK (hr_zone_2_max IS NULL OR hr_zone_2_max > 0),
    hr_zone_3_max INTEGER CHECK (hr_zone_3_max IS NULL OR hr_zone_3_max > 0),
    hr_zone_4_max INTEGER CHECK (hr_zone_4_max IS NULL OR hr_zone_4_max > 0),
    hr_zone_5_max INTEGER CHECK (hr_zone_5_max IS NULL OR hr_zone_5_max > 0),

    power_zone_1_max INTEGER CHECK (power_zone_1_max IS NULL OR power_zone_1_max > 0),
    power_zone_2_max INTEGER CHECK (power_zone_2_max IS NULL OR power_zone_2_max > 0),
    power_zone_3_max INTEGER CHECK (power_zone_3_max IS NULL OR power_zone_3_max > 0),
    power_zone_4_max INTEGER CHECK (power_zone_4_max IS NULL OR power_zone_4_max > 0),
    power_zone_5_max INTEGER CHECK (power_zone_5_max IS NULL OR power_zone_5_max > 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT athlete_config_singleton
        CHECK (id = '00000000-0000-0000-0000-000000000001'::uuid)
);
