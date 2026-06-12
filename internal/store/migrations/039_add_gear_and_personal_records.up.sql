-- add-garmin-gear-and-prs (change D): mirror two slowly-changing Garmin
-- inventory datasets — gear (shoe/bike mileage + retirement) and personal
-- records — as coaching context for the chat agent. Neither feeds fueling math.
-- Unlike the date-keyed snapshots, these are keyed by a stable Garmin id
-- (external_id, UNIQUE) and upserted in place, never appended per day.

CREATE TABLE gear (
    id               UUID PRIMARY KEY,
    external_id      TEXT NOT NULL,
    gear_type        TEXT NOT NULL CHECK (gear_type IN ('shoes', 'bike', 'other')),
    display_name     TEXT NOT NULL,
    total_distance_m NUMERIC(12, 1) CHECK (total_distance_m IS NULL OR total_distance_m >= 0),
    total_activities INTEGER        CHECK (total_activities IS NULL OR total_activities >= 0),
    retired          BOOLEAN NOT NULL DEFAULT false,
    date_begin       DATE,
    date_end         DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX gear_external_id_uidx ON gear (external_id);

CREATE TABLE personal_records (
    id          UUID PRIMARY KEY,
    external_id TEXT NOT NULL,
    pr_type     TEXT NOT NULL,
    value       NUMERIC(12, 3) NOT NULL CHECK (value >= 0),
    unit        TEXT NOT NULL,
    activity_id TEXT,
    achieved_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX personal_records_external_id_uidx ON personal_records (external_id);
