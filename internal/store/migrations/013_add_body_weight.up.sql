CREATE TABLE body_weight_entries (
    id            UUID PRIMARY KEY,
    logged_at     TIMESTAMPTZ NOT NULL,
    weight_kg     NUMERIC(5, 2) NOT NULL CHECK (weight_kg > 0),
    body_fat_pct  NUMERIC(4, 2) NULL CHECK (
        body_fat_pct IS NULL OR (body_fat_pct >= 0 AND body_fat_pct <= 100)
    ),
    note          TEXT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX body_weight_entries_logged_at_idx ON body_weight_entries (logged_at);
