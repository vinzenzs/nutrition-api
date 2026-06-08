CREATE TABLE workouts (
    id              UUID PRIMARY KEY,
    external_id     TEXT,
    source          TEXT NOT NULL CHECK (source IN ('garmin', 'manual', 'other')),
    sport           TEXT NOT NULL CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'other')),
    name            TEXT,

    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ NOT NULL CHECK (ended_at > started_at),

    kcal_burned     NUMERIC(10, 1) CHECK (kcal_burned IS NULL OR kcal_burned > 0),
    avg_hr          INTEGER        CHECK (avg_hr IS NULL OR avg_hr > 0),
    tss             NUMERIC(10, 2) CHECK (tss IS NULL OR tss >= 0),
    notes           TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workouts_started_at_idx ON workouts (started_at);

-- Partial unique index lets multiple rows with NULL external_id coexist (manual
-- entries have no implicit dedup) while preventing duplicates from any sourced
-- writer (Garmin re-syncs UPSERT through this constraint).
CREATE UNIQUE INDEX workouts_external_id_uidx
    ON workouts (external_id)
    WHERE external_id IS NOT NULL;
