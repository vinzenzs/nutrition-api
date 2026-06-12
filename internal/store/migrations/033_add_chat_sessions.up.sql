-- Server-side persistence of chat conversations (add-chat-sessions).
-- A session is a conversation header; chat_messages holds its ordered turns at
-- full Anthropic content-block fidelity (jsonb), cascade-deleted with the session.

CREATE TABLE chat_sessions (
    id              UUID PRIMARY KEY,
    -- NULL = untitled. Set from the first user message on the opening turn, or
    -- via PATCH /chat/sessions/{id}.
    title           TEXT NULL CHECK (title IS NULL OR char_length(title) BETWEEN 1 AND 200),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Drives most-recent-first list ordering; bumped on every persisted turn.
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chat_messages (
    id         UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    -- Monotonic per-insert tiebreaker: turns written within one request share
    -- the transaction's created_at, so seq preserves their order on load.
    seq        BIGINT GENERATED ALWAYS AS IDENTITY,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    -- The verbatim Anthropic content value: a JSON string for plain user text,
    -- or a content-block array (text / tool_use / tool_result) for richer turns.
    content    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chat_messages_session_seq_idx ON chat_messages (session_id, seq);
