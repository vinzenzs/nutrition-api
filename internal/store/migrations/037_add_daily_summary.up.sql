-- add-garmin-daily-energy (change A): a date-keyed snapshot of Garmin's whole-day
-- energy/activity totals (active/resting/total kcal, steps, floors, intensity
-- minutes, distance), so non-workout movement (NEAT) becomes a first-class,
-- queryable signal. One row per calendar day, upserted in place via the date
-- primary key — same pattern as recovery_metrics / fitness_metrics. Every metric
-- is nullable: NULL means "the device did not report it that day," a meaningful
-- state, not a data-quality bug. Unit-isolated — these expenditure/activity
-- totals never merge into summary's nutrition Totals or EA's denominator.

CREATE TABLE daily_summary (
    date                       DATE PRIMARY KEY,
    active_kcal                INTEGER       CHECK (active_kcal IS NULL OR active_kcal >= 0),
    resting_kcal               INTEGER       CHECK (resting_kcal IS NULL OR resting_kcal >= 0),
    total_kcal                 INTEGER       CHECK (total_kcal IS NULL OR total_kcal >= 0),
    steps                      INTEGER       CHECK (steps IS NULL OR steps >= 0),
    floors                     INTEGER       CHECK (floors IS NULL OR floors >= 0),
    moderate_intensity_minutes INTEGER       CHECK (moderate_intensity_minutes IS NULL OR moderate_intensity_minutes >= 0),
    vigorous_intensity_minutes INTEGER       CHECK (vigorous_intensity_minutes IS NULL OR vigorous_intensity_minutes >= 0),
    distance_m                 NUMERIC(10, 1) CHECK (distance_m IS NULL OR distance_m >= 0),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);
