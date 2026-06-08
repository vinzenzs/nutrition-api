CREATE TABLE hydration_entries (
    id           UUID PRIMARY KEY,
    logged_at    TIMESTAMPTZ NOT NULL,
    quantity_ml  NUMERIC(10, 1) NOT NULL CHECK (quantity_ml > 0),
    note         TEXT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hydration_entries_logged_at_idx ON hydration_entries (logged_at);
