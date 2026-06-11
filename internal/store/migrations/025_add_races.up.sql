CREATE TABLE races (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    race_date   DATE NOT NULL,
    race_type   TEXT NULL,
    location    TEXT NULL,
    notes       TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE race_legs (
    id                     UUID PRIMARY KEY,
    race_id                UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE,
    ordinal                INT NOT NULL,
    discipline             TEXT NOT NULL CHECK (discipline IN ('swim', 'bike', 'run', 'transition', 'other')),
    distance_m             NUMERIC(10, 1) NULL CHECK (distance_m IS NULL OR distance_m > 0),
    expected_duration_min  INT NULL CHECK (expected_duration_min IS NULL OR expected_duration_min > 0),
    intensity              TEXT NULL,
    UNIQUE (race_id, ordinal)
);

CREATE INDEX races_race_date_idx ON races (race_date);
CREATE INDEX race_legs_race_id_idx ON race_legs (race_id);
