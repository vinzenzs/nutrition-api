CREATE TABLE idempotency_records (
    client_id           TEXT NOT NULL,
    method              TEXT NOT NULL,
    path                TEXT NOT NULL,
    idempotency_key     TEXT NOT NULL,

    status              INTEGER NOT NULL,
    response_body       BYTEA NOT NULL,
    request_body_hash   TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (client_id, method, path, idempotency_key)
);

CREATE INDEX idempotency_records_created_at_idx
    ON idempotency_records (created_at);
