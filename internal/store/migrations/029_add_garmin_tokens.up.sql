-- Single-row store for the garmin-bridge's opaque auth token blob, encrypted
-- at rest (AES-256-GCM; the key lives in config, never in the DB). The
-- id = 1 CHECK constraint encodes "one user, one token" (per add-garmin-auth-token).
CREATE TABLE garmin_tokens (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    ciphertext BYTEA NOT NULL,
    nonce      BYTEA NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
